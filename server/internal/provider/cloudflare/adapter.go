package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dns-hub/server/internal/provider"
)

const baseURL = "https://api.cloudflare.com/client/v4"

type Adapter struct {
	token  string
	client *http.Client
}

type apiResponse[T any] struct {
	Success bool      `json:"success"`
	Errors  []cfError `json:"errors"`
	Result  T         `json:"result"`
}

type cfError struct {
	Message string `json:"message"`
}

type zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type dnsRecord struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority"`
	Proxied  *bool  `json:"proxied"`
	Comment  string `json:"comment"`
}

func init() {
	provider.MustRegister("cloudflare", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "cloudflare",
		Label:       "Cloudflare",
		Description: "Cloudflare DNS API",
		Fields: []provider.FieldSpec{
			{Key: "api_token", Label: "API Token", Type: provider.FieldTypePassword, Required: true, Placeholder: "Cloudflare API Token"},
		},
		SampleConfig: map[string]any{
			"api_token": "",
		},
	})
}

func New(config map[string]any) (*Adapter, error) {
	token, _ := config["api_token"].(string)
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("cloudflare api_token is required")
	}

	return &Adapter{
		token:  token,
		client: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "cloudflare"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "cloudflare credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	var response apiResponse[[]zone]
	if err := a.request(ctx, http.MethodGet, "/zones", nil, &response); err != nil {
		return nil, err
	}
	items := make([]provider.Domain, 0, len(response.Result))
	for _, item := range response.Result {
		items = append(items, provider.Domain{ZoneID: item.ID, Name: item.Name, Provider: a.Name()})
	}
	return items, nil
}

func (a *Adapter) ListRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	var response apiResponse[[]dnsRecord]
	path := fmt.Sprintf("/zones/%s/dns_records", url.PathEscape(zoneID))
	if err := a.request(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	items := make([]provider.DNSRecord, 0, len(response.Result))
	for _, item := range response.Result {
		items = append(items, mapRecord(item))
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	payload := map[string]any{
		"type":    strings.ToUpper(strings.TrimSpace(input.Type)),
		"name":    strings.TrimSpace(input.Name),
		"content": strings.TrimSpace(input.Content),
		"ttl":     input.TTL,
	}
	if input.Priority != nil {
		payload["priority"] = *input.Priority
	}
	if input.Proxied != nil {
		payload["proxied"] = *input.Proxied
	}
	if input.Comment != "" {
		payload["comment"] = input.Comment
	}

	var response apiResponse[dnsRecord]
	path := fmt.Sprintf("/zones/%s/dns_records", url.PathEscape(zoneID))
	method := http.MethodPost
	if strings.TrimSpace(input.ID) != "" {
		path = fmt.Sprintf("%s/%s", path, url.PathEscape(input.ID))
		method = http.MethodPut
	}
	if err := a.request(ctx, method, path, payload, &response); err != nil {
		return nil, err
	}
	result := mapRecord(response.Result)
	return &result, nil
}

func (a *Adapter) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", url.PathEscape(zoneID), url.PathEscape(recordID))
	var response apiResponse[map[string]any]
	return a.request(ctx, http.MethodDelete, path, nil, &response)
}

func (a *Adapter) ExportConfig() map[string]any {
	return map[string]any{
		"api_token": a.token,
	}
}

func (a *Adapter) request(ctx context.Context, method, path string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal cloudflare request: %w", err)
		}
		reader = bytes.NewBuffer(payload)
	}

	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build cloudflare request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+a.token)
	request.Header.Set("Content-Type", "application/json")

	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call cloudflare api: %w", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read cloudflare response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("cloudflare api returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode cloudflare response: %w", err)
	}

	switch typed := target.(type) {
	case *apiResponse[[]zone]:
		if !typed.Success {
			return fmt.Errorf("cloudflare api error: %s", joinErrors(typed.Errors))
		}
	case *apiResponse[[]dnsRecord]:
		if !typed.Success {
			return fmt.Errorf("cloudflare api error: %s", joinErrors(typed.Errors))
		}
	case *apiResponse[dnsRecord]:
		if !typed.Success {
			return fmt.Errorf("cloudflare api error: %s", joinErrors(typed.Errors))
		}
	case *apiResponse[map[string]any]:
		if !typed.Success {
			return fmt.Errorf("cloudflare api error: %s", joinErrors(typed.Errors))
		}
	}

	return nil
}

func mapRecord(item dnsRecord) provider.DNSRecord {
	return provider.DNSRecord{
		ID:       item.ID,
		Type:     item.Type,
		Name:     item.Name,
		Content:  item.Content,
		TTL:      item.TTL,
		Priority: item.Priority,
		Proxied:  item.Proxied,
		Comment:  item.Comment,
	}
}

func joinErrors(items []cfError) string {
	if len(items) == 0 {
		return "unknown error"
	}
	messages := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Message) != "" {
			messages = append(messages, item.Message)
		}
	}
	if len(messages) == 0 {
		return "unknown error"
	}
	return strings.Join(messages, "; ")
}

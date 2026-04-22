package digitalocean

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dns-hub/server/internal/provider"
)

const baseURL = "https://api.digitalocean.com/v2"

type Adapter struct {
	token  string
	client *http.Client
}

type domainsResponse struct {
	Domains []domainItem `json:"domains"`
}

type domainItem struct {
	Name string `json:"name"`
}

type recordsResponse struct {
	DomainRecords []domainRecord `json:"domain_records"`
}

type recordResponse struct {
	DomainRecord domainRecord `json:"domain_record"`
}

type domainRecord struct {
	ID       int    `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority"`
}

func init() {
	provider.MustRegister("digitalocean", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "digitalocean",
		Label:       "DigitalOcean",
		Description: "DigitalOcean DNS API",
		Fields: []provider.FieldSpec{
			{Key: "api_token", Label: "API Token", Type: provider.FieldTypePassword, Required: true, Placeholder: "dop_v1_...", HelpText: "需要具备 domain:read / create / update / delete 权限"},
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
		return nil, fmt.Errorf("digitalocean api_token is required")
	}
	return &Adapter{
		token:  token,
		client: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "digitalocean"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "digitalocean credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	var response domainsResponse
	if err := a.request(ctx, http.MethodGet, "/domains", nil, &response); err != nil {
		return nil, err
	}
	items := make([]provider.Domain, 0, len(response.Domains))
	for _, item := range response.Domains {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		items = append(items, provider.Domain{ZoneID: name, Name: name, Provider: a.Name()})
	}
	return items, nil
}

func (a *Adapter) ListRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	domainName := strings.TrimSpace(zoneID)
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}
	var response recordsResponse
	path := fmt.Sprintf("/domains/%s/records?per_page=200", url.PathEscape(domainName))
	if err := a.request(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	items := make([]provider.DNSRecord, 0, len(response.DomainRecords))
	for _, item := range response.DomainRecords {
		items = append(items, mapRecord(item, domainName))
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	domainName := strings.TrimSpace(zoneID)
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}
	payload, err := mutationPayload(input, domainName)
	if err != nil {
		return nil, err
	}
	var response recordResponse
	path := fmt.Sprintf("/domains/%s/records", url.PathEscape(domainName))
	method := http.MethodPost
	if strings.TrimSpace(input.ID) != "" {
		path = fmt.Sprintf("%s/%s", path, url.PathEscape(strings.TrimSpace(input.ID)))
		method = http.MethodPut
	}
	if err := a.request(ctx, method, path, payload, &response); err != nil {
		return nil, err
	}
	result := mapRecord(response.DomainRecord, domainName)
	return &result, nil
}

func (a *Adapter) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	domainName := strings.TrimSpace(zoneID)
	trimmedRecordID := strings.TrimSpace(recordID)
	if domainName == "" {
		return fmt.Errorf("domain name is required")
	}
	if trimmedRecordID == "" {
		return fmt.Errorf("record id is required")
	}
	return a.request(ctx, http.MethodDelete, fmt.Sprintf("/domains/%s/records/%s", url.PathEscape(domainName), url.PathEscape(trimmedRecordID)), nil, nil)
}

func (a *Adapter) ExportConfig() map[string]any {
	return map[string]any{
		"api_token": a.token,
	}
}

func mutationPayload(input provider.RecordMutation, domainName string) (map[string]any, error) {
	recordType := strings.ToUpper(strings.TrimSpace(input.Type))
	if recordType == "" {
		return nil, fmt.Errorf("record type is required")
	}
	name := normalizeRecordName(strings.TrimSpace(input.Name), domainName)
	payload := map[string]any{
		"type": recordType,
		"name": name,
		"data": strings.TrimSpace(input.Content),
	}
	if payload["data"] == "" {
		return nil, fmt.Errorf("record content is required")
	}
	if input.TTL > 0 {
		payload["ttl"] = input.TTL
	}
	if input.Priority != nil && usesPriority(recordType) {
		payload["priority"] = *input.Priority
	}
	return payload, nil
}

func usesPriority(recordType string) bool {
	switch recordType {
	case "MX", "SRV", "CAA":
		return true
	default:
		return false
	}
}

func normalizeRecordName(name, domainName string) string {
	if name == "" || name == "@" {
		return "@"
	}
	trimmedDomain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domainName)), ".")
	candidate := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	if candidate == trimmedDomain {
		return "@"
	}
	suffix := "." + trimmedDomain
	if strings.HasSuffix(candidate, suffix) {
		short := strings.TrimSuffix(candidate, suffix)
		short = strings.TrimSuffix(short, ".")
		if short == "" {
			return "@"
		}
		return short
	}
	return strings.TrimSpace(name)
}

func mapRecord(item domainRecord, domainName string) provider.DNSRecord {
	return provider.DNSRecord{
		ID:       strconv.Itoa(item.ID),
		Type:     strings.ToUpper(strings.TrimSpace(item.Type)),
		Name:     normalizeRecordName(item.Name, domainName),
		Content:  strings.TrimSpace(item.Data),
		TTL:      item.TTL,
		Priority: item.Priority,
	}
}

func (a *Adapter) request(ctx context.Context, method, path string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal digitalocean request: %w", err)
		}
		reader = bytes.NewBuffer(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build digitalocean request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+a.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call digitalocean api: %w", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read digitalocean response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("digitalocean api returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	if target == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode digitalocean response: %w", err)
	}
	return nil
}

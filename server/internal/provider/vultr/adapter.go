package vultr

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

const baseURL = "https://api.vultr.com/v2"

type Adapter struct {
	token  string
	client *http.Client
}

type domainsResponse struct {
	Domains []domainItem `json:"domains"`
}

type domainItem struct {
	Domain string `json:"domain"`
}

type recordsResponse struct {
	Records []domainRecord `json:"records"`
}

type recordResponse struct {
	Record domainRecord `json:"record"`
}

type domainRecord struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority"`
}

func init() {
	provider.MustRegister("vultr", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "vultr",
		Label:       "Vultr",
		Description: "Vultr DNS API",
		Fields: []provider.FieldSpec{
			{Key: "api_token", Label: "API Token", Type: provider.FieldTypePassword, Required: true, Placeholder: "Vultr API Token", HelpText: "需要具备 DNS 读写权限"},
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
		return nil, fmt.Errorf("vultr api_token is required")
	}
	return &Adapter{
		token:  token,
		client: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "vultr"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "vultr credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	var response domainsResponse
	if err := a.request(ctx, http.MethodGet, "/domains?per_page=500", nil, &response); err != nil {
		return nil, err
	}
	items := make([]provider.Domain, 0, len(response.Domains))
	for _, item := range response.Domains {
		name := strings.TrimSpace(item.Domain)
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
	path := fmt.Sprintf("/domains/%s/records?per_page=500", url.PathEscape(domainName))
	if err := a.request(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	items := make([]provider.DNSRecord, 0, len(response.Records))
	for _, item := range response.Records {
		items = append(items, mapRecord(item, domainName))
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	domainName := strings.TrimSpace(zoneID)
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}
	if strings.TrimSpace(input.ID) == "" {
		payload, err := createPayload(input, domainName)
		if err != nil {
			return nil, err
		}
		var response recordResponse
		path := fmt.Sprintf("/domains/%s/records", url.PathEscape(domainName))
		if err := a.request(ctx, http.MethodPost, path, payload, &response); err != nil {
			return nil, err
		}
		result := mapRecord(response.Record, domainName)
		return &result, nil
	}

	records, err := a.ListRecords(ctx, domainName)
	if err != nil {
		return nil, err
	}
	current := findRecord(records, strings.TrimSpace(input.ID))
	if current == nil {
		return nil, fmt.Errorf("record %s not found", strings.TrimSpace(input.ID))
	}
	payload, err := updatePayload(*current, input, domainName)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/domains/%s/records/%s", url.PathEscape(domainName), url.PathEscape(strings.TrimSpace(input.ID)))
	if err := a.request(ctx, http.MethodPatch, path, payload, nil); err != nil {
		return nil, err
	}
	updated := mergeRecord(*current, input, domainName)
	return &updated, nil
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

func createPayload(input provider.RecordMutation, domainName string) (map[string]any, error) {
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

func updatePayload(current provider.DNSRecord, input provider.RecordMutation, domainName string) (map[string]any, error) {
	updated := mergeRecord(current, input, domainName)
	payload := map[string]any{
		"name": normalizeRecordName(updated.Name, domainName),
		"data": strings.TrimSpace(updated.Content),
	}
	if payload["data"] == "" {
		return nil, fmt.Errorf("record content is required")
	}
	if updated.TTL > 0 {
		payload["ttl"] = updated.TTL
	}
	if updated.Priority != nil && usesPriority(strings.ToUpper(strings.TrimSpace(updated.Type))) {
		payload["priority"] = *updated.Priority
	}
	return payload, nil
}

func mergeRecord(current provider.DNSRecord, input provider.RecordMutation, domainName string) provider.DNSRecord {
	updated := current
	if strings.TrimSpace(input.Type) != "" {
		updated.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	}
	if input.Name != "" {
		updated.Name = normalizeRecordName(strings.TrimSpace(input.Name), domainName)
	}
	if strings.TrimSpace(input.Content) != "" {
		updated.Content = strings.TrimSpace(input.Content)
	}
	if input.TTL > 0 {
		updated.TTL = input.TTL
	}
	if input.Priority != nil {
		updated.Priority = input.Priority
	}
	return updated
}

func findRecord(records []provider.DNSRecord, recordID string) *provider.DNSRecord {
	for index := range records {
		if strings.TrimSpace(records[index].ID) == recordID {
			return &records[index]
		}
	}
	return nil
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
		ID:       strings.TrimSpace(item.ID),
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
			return fmt.Errorf("marshal vultr request: %w", err)
		}
		reader = bytes.NewBuffer(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build vultr request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+a.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call vultr api: %w", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read vultr response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("vultr api returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	if target == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode vultr response: %w", err)
	}
	return nil
}

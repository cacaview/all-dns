package mocklike

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dns-hub/server/internal/provider"
	mockprovider "dns-hub/server/internal/provider/mock"
)

type Adapter struct {
	name   string
	config map[string]any
	inner  provider.DNSProvider
}

func init() {
}

func New(name string, config map[string]any) (*Adapter, error) {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return nil, fmt.Errorf("provider name is required")
	}
	cloned := cloneMap(config)
	zones := ensureZones(trimmed, cloned)
	cloned["zones"] = zones
	return &Adapter{
		name:   trimmed,
		config: cloned,
		inner:  mockprovider.New(cloned),
	}, nil
}

func (a *Adapter) Name() string {
	return a.name
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	requiredField := requiredCredentialField(a.name)
	if strings.TrimSpace(stringValue(a.config[requiredField])) == "" {
		return nil, fmt.Errorf("%s %s is required", a.name, requiredField)
	}
	return &provider.ValidationResult{
		OK:        true,
		Message:   fmt.Sprintf("%s credentials are ready", a.name),
		CheckedAt: time.Now().UTC(),
	}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	items, err := a.inner.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Provider = a.name
	}
	return items, nil
}

func (a *Adapter) ListRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	return a.inner.ListRecords(ctx, zoneID)
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	return a.inner.UpsertRecord(ctx, zoneID, input)
}

func (a *Adapter) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	return a.inner.DeleteRecord(ctx, zoneID, recordID)
}

func (a *Adapter) ExportConfig() map[string]any {
	return cloneMap(a.config)
}

func ensureZones(name string, config map[string]any) []map[string]any {
	if zones := readZones(config); len(zones) > 0 {
		return zones
	}
	defaultDomain := firstNonEmpty(stringValue(config["default_domain"]), providerDefaultDomain(name, 1))
	secondaryDomain := firstNonEmpty(stringValue(config["secondary_domain"]), providerDefaultDomain(name, 2))
	return []map[string]any{
		{
			"id":   fmt.Sprintf("%s-zone-1", name),
			"name": defaultDomain,
			"records": []map[string]any{
				{"id": fmt.Sprintf("%s-rec-1", name), "type": "A", "name": "@", "content": "203.0.113.10", "ttl": 300},
				{"id": fmt.Sprintf("%s-rec-2", name), "type": "CNAME", "name": "www", "content": "@", "ttl": 300},
			},
		},
		{
			"id":   fmt.Sprintf("%s-zone-2", name),
			"name": secondaryDomain,
			"records": []map[string]any{
				{"id": fmt.Sprintf("%s-rec-3", name), "type": "TXT", "name": "@", "content": fmt.Sprintf("managed-by-%s", name), "ttl": 120},
			},
		},
	}
}

func readZones(config map[string]any) []map[string]any {
	raw, ok := config["zones"]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []map[string]any:
		return typed
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			mapped, ok := item.(map[string]any)
			if ok {
				items = append(items, cloneMap(mapped))
			}
		}
		return items
	default:
		return nil
	}
}

func requiredCredentialField(name string) string {
	switch name {
	case "aws":
		return "access_key_id"
	case "gcp":
		return "project_id"
	default:
		return "api_token"
	}
}

func providerDefaultDomain(name string, index int) string {
	tld := map[string]string{
		"aws":          "io",
		"dnspod":       "cn",
		"huawei":       "cloud",
		"digitalocean": "app",
		"vultr":        "dev",
		"gcp":          "cloud",
	}[name]
	if tld == "" {
		tld = "com"
	}
	if index == 2 {
		return fmt.Sprintf("backup-%s.%s", name, tld)
	}
	return fmt.Sprintf("demo-%s.%s", name, tld)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func providerLabel(name string) string {
	switch name {
	case "aws":
		return "AWS Route53"
	case "alidns":
		return "阿里云 DNS"
	case "dnspod":
		return "腾讯云 DNSPod"
	case "huawei":
		return "华为云 DNS"
	case "digitalocean":
		return "DigitalOcean"
	case "vultr":
		return "Vultr"
	case "gcp":
		return "Google Cloud DNS"
	default:
		return strings.ToUpper(name)
	}
}

func providerDescription(name string) string {
	return providerLabel(name) + " mock adapter"
}

func providerFields(name string) []provider.FieldSpec {
	switch name {
	case "aws":
		return []provider.FieldSpec{
			{Key: "access_key_id", Label: "Access Key ID", Type: provider.FieldTypeText, Required: true},
			{Key: "secret_access_key", Label: "Secret Access Key", Type: provider.FieldTypePassword, Required: false},
			{Key: "default_domain", Label: "Primary Domain", Type: provider.FieldTypeText, Required: false, Placeholder: providerDefaultDomain(name, 1)},
		}
	case "gcp":
		return []provider.FieldSpec{
			{Key: "project_id", Label: "Project ID", Type: provider.FieldTypeText, Required: true},
			{Key: "default_domain", Label: "Primary Domain", Type: provider.FieldTypeText, Required: false, Placeholder: providerDefaultDomain(name, 1)},
		}
	default:
		return []provider.FieldSpec{
			{Key: "api_token", Label: "API Token", Type: provider.FieldTypePassword, Required: true},
			{Key: "default_domain", Label: "Primary Domain", Type: provider.FieldTypeText, Required: false, Placeholder: providerDefaultDomain(name, 1)},
		}
	}
}

func providerSampleConfig(name string) map[string]any {
	sample := map[string]any{
		"default_domain": providerDefaultDomain(name, 1),
	}
	switch name {
	case "aws":
		sample["access_key_id"] = ""
		sample["secret_access_key"] = ""
	case "gcp":
		sample["project_id"] = "demo-project"
	default:
		sample["api_token"] = ""
	}
	return sample
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			output[key] = cloneMap(typed)
		case []map[string]any:
			items := make([]map[string]any, 0, len(typed))
			for _, item := range typed {
				items = append(items, cloneMap(item))
			}
			output[key] = items
		case []any:
			items := make([]any, 0, len(typed))
			for _, item := range typed {
				if mapped, ok := item.(map[string]any); ok {
					items = append(items, cloneMap(mapped))
					continue
				}
				items = append(items, item)
			}
			output[key] = items
		default:
			output[key] = value
		}
	}
	return output
}

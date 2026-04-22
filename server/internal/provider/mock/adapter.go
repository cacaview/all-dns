package mock

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dns-hub/server/internal/provider"
)

type Adapter struct {
	config map[string]any
}

func init() {
	provider.MustRegister("mock", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config), nil
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "mock",
		Label:       "Mock",
		Description: "Local mock provider for demos and development",
		Fields: []provider.FieldSpec{
			{Key: "default_domain", Label: "Primary Domain", Type: provider.FieldTypeText, Required: false, Placeholder: "example.com"},
			{Key: "secondary_domain", Label: "Secondary Domain", Type: provider.FieldTypeText, Required: false, Placeholder: "example.net"},
		},
		SampleConfig: map[string]any{
			"default_domain":   "example.com",
			"secondary_domain": "example.net",
		},
	})
}

func New(config map[string]any) *Adapter {
	adapter := &Adapter{config: cloneMap(config)}
	adapter.ensureDefaults()
	return adapter
}

func (a *Adapter) Name() string {
	return "mock"
}

func (a *Adapter) Validate(context.Context) (*provider.ValidationResult, error) {
	return &provider.ValidationResult{
		OK:        true,
		Message:   "mock provider is ready",
		CheckedAt: time.Now().UTC(),
	}, nil
}

func (a *Adapter) ListDomains(context.Context) ([]provider.Domain, error) {
	zones := getZoneMaps(a.config)
	items := make([]provider.Domain, 0, len(zones))
	for _, zone := range zones {
		items = append(items, provider.Domain{
			ZoneID:   getString(zone, "id", ""),
			Name:     getString(zone, "name", ""),
			Provider: a.Name(),
		})
	}
	return items, nil
}

func (a *Adapter) ListRecords(_ context.Context, zoneID string) ([]provider.DNSRecord, error) {
	zone := a.findZone(zoneID)
	if zone == nil {
		return nil, fmt.Errorf("zone %s not found", zoneID)
	}

	records := getRecordMaps(zone)
	items := make([]provider.DNSRecord, 0, len(records))
	for _, record := range records {
		items = append(items, mapRecord(record))
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(_ context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	zone := a.findZone(zoneID)
	if zone == nil {
		return nil, fmt.Errorf("zone %s not found", zoneID)
	}

	records := getRecordMaps(zone)
	for index, record := range records {
		if getString(record, "id", "") == input.ID && input.ID != "" {
			records[index] = mutationToMap(input)
			zone["records"] = records
			result := mapRecord(records[index])
			return &result, nil
		}
	}

	if input.ID == "" {
		input.ID = fmt.Sprintf("mock-%d", time.Now().UnixNano())
	}
	recordMap := mutationToMap(input)
	records = append(records, recordMap)
	zone["records"] = records
	result := mapRecord(recordMap)
	return &result, nil
}

func (a *Adapter) DeleteRecord(_ context.Context, zoneID string, recordID string) error {
	zone := a.findZone(zoneID)
	if zone == nil {
		return fmt.Errorf("zone %s not found", zoneID)
	}

	records := getRecordMaps(zone)
	filtered := make([]map[string]any, 0, len(records))
	removed := false
	for _, record := range records {
		if getString(record, "id", "") == recordID {
			removed = true
			continue
		}
		filtered = append(filtered, record)
	}
	if !removed {
		return fmt.Errorf("record %s not found", recordID)
	}
	zone["records"] = filtered
	return nil
}

func (a *Adapter) ExportConfig() map[string]any {
	return cloneMap(a.config)
}

func (a *Adapter) ensureDefaults() {
	if _, ok := a.config["zones"]; ok {
		return
	}

	a.config["zones"] = []map[string]any{
		{
			"id":   "mock-zone-1",
			"name": getString(a.config, "default_domain", "example.com"),
			"records": []map[string]any{
				{"id": "rec-1", "type": "A", "name": "@", "content": "203.0.113.10", "ttl": 300},
				{"id": "rec-2", "type": "CNAME", "name": "www", "content": "@", "ttl": 300},
			},
		},
		{
			"id":   "mock-zone-2",
			"name": getString(a.config, "secondary_domain", "example.net"),
			"records": []map[string]any{
				{"id": "rec-3", "type": "TXT", "name": "@", "content": "mock-managed", "ttl": 120},
			},
		},
	}
}

func (a *Adapter) findZone(zoneID string) map[string]any {
	zones := getZoneMaps(a.config)
	for _, zone := range zones {
		if getString(zone, "id", "") == zoneID {
			return zone
		}
	}
	return nil
}

func getZoneMaps(config map[string]any) []map[string]any {
	raw, ok := config["zones"]
	if !ok {
		return nil
	}

	switch value := raw.(type) {
	case []map[string]any:
		return value
	case []any:
		items := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if zone, ok := item.(map[string]any); ok {
				items = append(items, zone)
			}
		}
		return items
	default:
		return nil
	}
}

func getRecordMaps(zone map[string]any) []map[string]any {
	raw, ok := zone["records"]
	if !ok {
		return []map[string]any{}
	}

	switch value := raw.(type) {
	case []map[string]any:
		return value
	case []any:
		items := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if record, ok := item.(map[string]any); ok {
				items = append(items, record)
			}
		}
		return items
	default:
		return []map[string]any{}
	}
}

func mutationToMap(input provider.RecordMutation) map[string]any {
	item := map[string]any{
		"id":      input.ID,
		"type":    strings.ToUpper(strings.TrimSpace(input.Type)),
		"name":    strings.TrimSpace(input.Name),
		"content": strings.TrimSpace(input.Content),
		"ttl":     input.TTL,
	}
	if input.Priority != nil {
		item["priority"] = *input.Priority
	}
	if input.Proxied != nil {
		item["proxied"] = *input.Proxied
	}
	if input.Comment != "" {
		item["comment"] = input.Comment
	}
	return item
}

func mapRecord(record map[string]any) provider.DNSRecord {
	result := provider.DNSRecord{
		ID:      getString(record, "id", ""),
		Type:    getString(record, "type", "A"),
		Name:    getString(record, "name", "@"),
		Content: getString(record, "content", ""),
		TTL:     getInt(record, "ttl", 300),
		Comment: getString(record, "comment", ""),
	}
	if value, ok := record["priority"]; ok {
		priority := getAnyInt(value, 10)
		result.Priority = &priority
	}
	if value, ok := record["proxied"]; ok {
		proxied, ok := value.(bool)
		if ok {
			result.Proxied = &proxied
		}
	}
	return result
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

func getString(input map[string]any, key string, fallback string) string {
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}
	text, ok := value.(string)
	if !ok {
		return fallback
	}
	if strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}

func getInt(input map[string]any, key string, fallback int) int {
	value, ok := input[key]
	if !ok || value == nil {
		return fallback
	}
	return getAnyInt(value, fallback)
}

func getAnyInt(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

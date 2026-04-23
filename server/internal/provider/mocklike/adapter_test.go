package mocklike

import (
	"context"
	"testing"

	"dns-hub/server/internal/provider"
)

func TestAdapter_New(t *testing.T) {
	adapter, err := New("cloudflare", map[string]any{"api_token": "test-token"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if adapter.Name() != "cloudflare" {
		t.Errorf("expected name 'cloudflare', got %q", adapter.Name())
	}
}

func TestAdapter_New_EmptyName(t *testing.T) {
	_, err := New("", map[string]any{"api_token": "test"})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestAdapter_Validate(t *testing.T) {
	adapter, _ := New("cloudflare", map[string]any{"api_token": "valid-token"})
	result, err := adapter.Validate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Error("expected OK=true for valid token")
	}
}

func TestAdapter_Validate_MissingToken(t *testing.T) {
	adapter, _ := New("cloudflare", map[string]any{"api_token": ""})
	_, err := adapter.Validate(context.Background())
	if err == nil {
		t.Error("expected error for missing token")
	}
}

func TestAdapter_ListDomains(t *testing.T) {
	adapter, _ := New("aws", map[string]any{
		"access_key_id": "key",
		"default_domain": "example.com",
	})
	domains, err := adapter.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(domains) == 0 {
		t.Error("expected at least one domain")
	}
	if domains[0].Provider != "aws" {
		t.Errorf("expected provider 'aws', got %q", domains[0].Provider)
	}
}

func TestAdapter_ListRecords(t *testing.T) {
	adapter, _ := New("dnspod", map[string]any{
		"api_token": "token",
		"default_domain": "test.cn",
	})
	domains, _ := adapter.ListDomains(context.Background())
	if len(domains) == 0 {
		t.Fatal("no domains available")
	}
	records, err := adapter.ListRecords(context.Background(), domains[0].ZoneID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) == 0 {
		t.Error("expected at least one record")
	}
}

func TestAdapter_UpsertRecord(t *testing.T) {
	adapter, _ := New("vultr", map[string]any{"api_token": "tok"})
	domains, _ := adapter.ListDomains(context.Background())
	if len(domains) == 0 {
		t.Fatal("no domains available")
	}
	mut := provider.RecordMutation{Type: "A", Name: "www", Content: "1.2.3.4", TTL: 300}
	record, err := adapter.UpsertRecord(context.Background(), domains[0].ZoneID, mut)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Content != "1.2.3.4" {
		t.Errorf("expected content '1.2.3.4', got %q", record.Content)
	}
}

func TestAdapter_DeleteRecord(t *testing.T) {
	adapter, _ := New("digitalocean", map[string]any{"api_token": "tok"})
	domains, _ := adapter.ListDomains(context.Background())
	if len(domains) == 0 {
		t.Fatal("no domains available")
	}
	// Insert a record then delete it
	mut := provider.RecordMutation{Type: "TXT", Name: "test", Content: "hello", TTL: 120}
	record, _ := adapter.UpsertRecord(context.Background(), domains[0].ZoneID, mut)
	err := adapter.DeleteRecord(context.Background(), domains[0].ZoneID, record.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdapter_ExportConfig(t *testing.T) {
	adapter, _ := New("huawei", map[string]any{"api_token": "secret-token"})
	cfg := adapter.ExportConfig()
	if cfg["api_token"] != "secret-token" {
		t.Errorf("expected token 'secret-token', got %v", cfg["api_token"])
	}
}

func TestAdapter_CloneMap(t *testing.T) {
	original := map[string]any{
		"key": "value",
		"nested": map[string]any{"a": "b"},
		"list": []any{"x", "y"},
	}
	cloned := cloneMap(original)
	if cloned["key"] != "value" {
		t.Errorf("expected 'value', got %v", cloned["key"])
	}
	nested := cloned["nested"].(map[string]any)
	if nested["a"] != "b" {
		t.Errorf("expected 'b', got %v", nested["a"])
	}
	// Verify deep copy — modifying cloned doesn't affect original
	nested["a"] = "changed"
	origNested := original["nested"].(map[string]any)
	if origNested["a"] != "b" {
		t.Error("cloneMap did not deep-copy nested map")
	}
}

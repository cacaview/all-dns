package mock

import (
	"context"
	"testing"

	"dns-hub/server/internal/provider"
)

func TestAdapter_New(t *testing.T) {
	adapter := New(map[string]any{"default_domain": "test.example.com"})
	if adapter.Name() != "mock" {
		t.Errorf("expected name 'mock', got %q", adapter.Name())
	}
}

func TestAdapter_Validate(t *testing.T) {
	adapter := New(nil)
	result, err := adapter.Validate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Error("expected OK=true")
	}
}

func TestAdapter_ListDomains(t *testing.T) {
	adapter := New(map[string]any{
		"default_domain":   "primary.com",
		"secondary_domain": "secondary.com",
	})
	domains, err := adapter.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(domains))
	}
	if domains[0].Provider != "mock" {
		t.Errorf("expected provider 'mock', got %q", domains[0].Provider)
	}
}

func TestAdapter_ListRecords(t *testing.T) {
	adapter := New(map[string]any{
		"default_domain": "test.example.com",
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
		t.Error("expected at least one record in default zone")
	}
}

func TestAdapter_ListRecords_UnknownZone(t *testing.T) {
	adapter := New(nil)
	_, err := adapter.ListRecords(context.Background(), "nonexistent-zone")
	if err == nil {
		t.Error("expected error for unknown zone")
	}
}

func TestAdapter_UpsertRecord_New(t *testing.T) {
	adapter := New(nil)
	domains, _ := adapter.ListDomains(context.Background())
	if len(domains) == 0 {
		t.Fatal("no domains available")
	}
	mut := provider.RecordMutation{Type: "A", Name: "www", Content: "5.6.7.8", TTL: 600}
	record, err := adapter.UpsertRecord(context.Background(), domains[0].ZoneID, mut)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Content != "5.6.7.8" {
		t.Errorf("expected content '5.6.7.8', got %q", record.Content)
	}
}

func TestAdapter_UpsertRecord_Update(t *testing.T) {
	adapter := New(nil)
	domains, _ := adapter.ListDomains(context.Background())
	if len(domains) == 0 {
		t.Fatal("no domains available")
	}
	// Insert first
	mut := provider.RecordMutation{Type: "A", Name: "www", Content: "1.2.3.4", TTL: 300}
	inserted, _ := adapter.UpsertRecord(context.Background(), domains[0].ZoneID, mut)
	// Update with same ID
	mut.ID = inserted.ID
	mut.Content = "9.9.9.9"
	updated, err := adapter.UpsertRecord(context.Background(), domains[0].ZoneID, mut)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Content != "9.9.9.9" {
		t.Errorf("expected updated content '9.9.9.9', got %q", updated.Content)
	}
}

func TestAdapter_DeleteRecord(t *testing.T) {
	adapter := New(nil)
	domains, _ := adapter.ListDomains(context.Background())
	if len(domains) == 0 {
		t.Fatal("no domains available")
	}
	mut := provider.RecordMutation{Type: "TXT", Name: "to-delete", Content: "delete me", TTL: 120}
	record, _ := adapter.UpsertRecord(context.Background(), domains[0].ZoneID, mut)
	err := adapter.DeleteRecord(context.Background(), domains[0].ZoneID, record.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify deleted
	records, _ := adapter.ListRecords(context.Background(), domains[0].ZoneID)
	for _, r := range records {
		if r.ID == record.ID {
			t.Error("record should have been deleted")
		}
	}
}

func TestAdapter_DeleteRecord_NotFound(t *testing.T) {
	adapter := New(nil)
	domains, _ := adapter.ListDomains(context.Background())
	if len(domains) == 0 {
		t.Fatal("no domains available")
	}
	err := adapter.DeleteRecord(context.Background(), domains[0].ZoneID, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent record")
	}
}

func TestAdapter_ExportConfig(t *testing.T) {
	adapter := New(map[string]any{"default_domain": "export-test.com"})
	cfg := adapter.ExportConfig()
	if cfg["default_domain"] != "export-test.com" {
		t.Errorf("expected default_domain 'export-test.com', got %v", cfg["default_domain"])
	}
}

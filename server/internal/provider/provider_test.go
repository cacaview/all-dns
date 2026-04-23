package provider

import (
	"context"
	"testing"
)

func TestProviderRegistry_Register(t *testing.T) {
	// Test that we can register a new provider
	err := Register("testprovider", func(config map[string]any) (DNSProvider, error) {
		return &mockProvider{name: "testprovider"}, nil
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Verify it's in the registry
	providers := RegisteredProviders()
	found := false
	for _, p := range providers {
		if p == "testprovider" {
			found = true
			break
		}
	}
	if !found {
		t.Error("testprovider not found in registered providers")
	}
}

func TestProviderRegistry_MustRegister_Panic(t *testing.T) {
	// Register a name that already exists (testprovider registered in TestProviderRegistry_Register)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate registration")
		}
	}()
	// testprovider was already registered by TestProviderRegistry_Register
	MustRegister("testprovider", func(config map[string]any) (DNSProvider, error) {
		return nil, nil
	})
}

func TestProviderRegistry_New(t *testing.T) {
	// testprovider was registered in TestProviderRegistry_Register
	p, err := New("testprovider", map[string]any{"token": "test"})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}
	if p.Name() != "testprovider" {
		t.Errorf("expected name 'testprovider', got %q", p.Name())
	}
}

func TestProviderRegistry_New_Unknown(t *testing.T) {
	_, err := New("nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestProviderRegistry_RegisterDescriptor(t *testing.T) {
	// Register a new descriptor with a fresh key
	desc := Descriptor{
		Key:         "freshdesc",
		Label:       "Fresh Descriptor",
		Description: "A fresh test descriptor",
		Fields:      []FieldSpec{{Key: "token", Type: FieldTypePassword, Label: "Token", Required: true}},
	}
	err := RegisterDescriptor(desc)
	if err != nil {
		t.Fatalf("register descriptor failed: %v", err)
	}
	// Verify it can be retrieved via direct map access
	registryMu.RLock()
	d, ok := descriptors["freshdesc"]
	registryMu.RUnlock()
	if !ok {
		t.Fatal("freshdesc not found in descriptors map")
	}
	if d.Label != "Fresh Descriptor" {
		t.Errorf("expected label 'Fresh Descriptor', got %q", d.Label)
	}
}

func TestProviderRegistry_RegisterDescriptor_Duplicate(t *testing.T) {
	// Register a unique key first within this test
	desc := Descriptor{Key: "uniquedup", Label: "Unique"}
	err := RegisterDescriptor(desc)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	// Verify second registration with same key fails
	err = RegisterDescriptor(Descriptor{Key: "uniquedup", Label: "Duplicate"})
	if err == nil {
		t.Error("expected error for duplicate descriptor key 'uniquedup'")
	}
}

func TestRegisteredDescriptors_Sorted(t *testing.T) {
	descriptors := RegisteredDescriptors()
	for i := 1; i < len(descriptors); i++ {
		if descriptors[i].Key < descriptors[i-1].Key {
			t.Errorf("descriptors not sorted: %s < %s", descriptors[i].Key, descriptors[i-1].Key)
		}
	}
}

func TestMockProvider_Name(t *testing.T) {
	p := &mockProvider{name: "mymock"}
	if p.Name() != "mymock" {
		t.Errorf("expected 'mymock', got %q", p.Name())
	}
}

func TestMockProvider_Validate(t *testing.T) {
	p := &mockProvider{name: "mock", token: "valid"}
	result, err := p.Validate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Error("expected OK=true for valid token")
	}
	if result.Message != "mock: account is valid" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestMockProvider_Validate_InvalidToken(t *testing.T) {
	p := &mockProvider{name: "mock", token: "invalid"}
	result, err := p.Validate(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OK {
		t.Error("expected OK=false for invalid token")
	}
}

func TestMockProvider_ListDomains(t *testing.T) {
	p := &mockProvider{name: "mock", domains: []Domain{
		{ZoneID: "zone1", Name: "example.com", Provider: "mock"},
		{ZoneID: "zone2", Name: "test.org", Provider: "mock"},
	}}
	domains, err := p.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(domains))
	}
	if domains[0].Name != "example.com" {
		t.Errorf("expected first domain 'example.com', got %q", domains[0].Name)
	}
}

func TestMockProvider_ListRecords(t *testing.T) {
	p := &mockProvider{name: "mock", records: []DNSRecord{
		{ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4", TTL: 300},
		{ID: "r2", Type: "MX", Name: "@", Content: "mail.example.com", TTL: 600},
	}}
	records, err := p.ListRecords(context.Background(), "zone1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
	if records[0].Type != "A" {
		t.Errorf("expected first record type 'A', got %q", records[0].Type)
	}
}

func TestMockProvider_UpsertRecord(t *testing.T) {
	p := &mockProvider{name: "mock"}
	mut := RecordMutation{Type: "A", Name: "www", Content: "5.6.7.8", TTL: 300}
	record, err := p.UpsertRecord(context.Background(), "zone1", mut)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Content != "5.6.7.8" {
		t.Errorf("expected content '5.6.7.8', got %q", record.Content)
	}
	if record.Type != "A" {
		t.Errorf("expected type 'A', got %q", record.Type)
	}
}

func TestMockProvider_DeleteRecord(t *testing.T) {
	p := &mockProvider{name: "mock", records: []DNSRecord{
		{ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4"},
	}}
	err := p.DeleteRecord(context.Background(), "zone1", "r1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.records) != 0 {
		t.Errorf("expected 0 records after delete, got %d", len(p.records))
	}
}

func TestMockProvider_ExportConfig(t *testing.T) {
	p := &mockProvider{name: "mock", token: "secret"}
	cfg := p.ExportConfig()
	if cfg["token"] != "secret" {
		t.Errorf("expected token 'secret', got %v", cfg["token"])
	}
}

// mockProvider is a test double for DNSProvider.
type mockProvider struct {
	name    string
	token   string
	domains []Domain
	records []DNSRecord
}

func (m *mockProvider) Name() string                     { return m.name }
func (m *mockProvider) Validate(ctx context.Context) (*ValidationResult, error) {
	if m.token == "invalid" {
		return &ValidationResult{OK: false, Message: "invalid token"}, nil
	}
	return &ValidationResult{OK: true, Message: m.name + ": account is valid"}, nil
}
func (m *mockProvider) ListDomains(ctx context.Context) ([]Domain, error) { return m.domains, nil }
func (m *mockProvider) ListRecords(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	return m.records, nil
}
func (m *mockProvider) UpsertRecord(ctx context.Context, zoneID string, mut RecordMutation) (*DNSRecord, error) {
	record := DNSRecord{
		ID:      mut.ID,
		Type:    mut.Type,
		Name:    mut.Name,
		Content: mut.Content,
		TTL:     mut.TTL,
	}
	m.records = append(m.records, record)
	return &record, nil
}
func (m *mockProvider) DeleteRecord(ctx context.Context, zoneID, recordID string) error {
	for i, r := range m.records {
		if r.ID == recordID {
			m.records = append(m.records[:i], m.records[i+1:]...)
			break
		}
	}
	return nil
}
func (m *mockProvider) ExportConfig() map[string]any { return map[string]any{"token": m.token} }

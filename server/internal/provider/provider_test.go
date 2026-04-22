package provider

import (
	"testing"
)

func TestDNSRecord_Fields(t *testing.T) {
	priority := 0
	proxied := false
	record := DNSRecord{
		ID:       "abc123",
		Type:     "A",
		Name:     "www",
		Content:  "1.2.3.4",
		TTL:      300,
		Priority: &priority,
		Proxied:  &proxied,
		Comment:  "test record",
	}

	if record.ID != "abc123" {
		t.Errorf("expected ID=abc123, got %s", record.ID)
	}
	if record.Type != "A" {
		t.Errorf("expected Type=A, got %s", record.Type)
	}
	if record.Content != "1.2.3.4" {
		t.Errorf("expected Content=1.2.3.4, got %s", record.Content)
	}
}

func TestRecordMutation_Fields(t *testing.T) {
	priority := 10
	proxied := true
	mut := RecordMutation{
		ID:       "record-1",
		Type:     "CNAME",
		Name:     "blog",
		Content:  "example.com",
		TTL:      600,
		Priority: &priority,
		Proxied:  &proxied,
		Comment:  "blog cname",
	}

	if mut.Type != "CNAME" {
		t.Errorf("expected CNAME, got %s", mut.Type)
	}
	if *mut.Priority != 10 {
		t.Errorf("expected Priority=10, got %d", *mut.Priority)
	}
}

func TestValidationResult_Fields(t *testing.T) {
	result := ValidationResult{
		OK:      true,
		Message: "account is valid",
	}

	if !result.OK {
		t.Error("expected OK=true")
	}
	if result.Message != "account is valid" {
		t.Errorf("expected message 'account is valid', got %s", result.Message)
	}
}

func TestFieldSpec_Fields(t *testing.T) {
	spec := FieldSpec{
		Key:         "api_token",
		Type:       "password",
		Label:      "API Token",
		Required:   true,
		HelpText:   "Your Cloudflare API token",
	}

	if spec.Key != "api_token" {
		t.Errorf("expected key=api_token, got %s", spec.Key)
	}
	if spec.Type != "password" {
		t.Errorf("expected type=password, got %s", spec.Type)
	}
	if !spec.Required {
		t.Error("expected Required=true")
	}
}

func TestDescriptor_Fields(t *testing.T) {
	desc := Descriptor{
		Key:   "cloudflare",
		Label: "Cloudflare",
		Description: "Cloudflare DNS",
		Fields: []FieldSpec{},
	}

	if desc.Key != "cloudflare" {
		t.Errorf("expected Key=cloudflare, got %s", desc.Key)
	}
	if desc.Label != "Cloudflare" {
		t.Errorf("expected Label=Cloudflare, got %s", desc.Label)
	}
	if desc.Description != "Cloudflare DNS" {
		t.Errorf("expected Description=Cloudflare DNS, got %s", desc.Description)
	}
}

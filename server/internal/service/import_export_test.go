package service

import (
	"bytes"
	"encoding/json"
	"testing"

	"dns-hub/server/internal/provider"
)

func TestParseRecordsJSON_Valid(t *testing.T) {
	content := []byte(`[
		{"type":"A","name":"www","content":"1.2.3.4","ttl":300},
		{"type":"MX","name":"@","content":"mail.example.com","ttl":600,"priority":10}
	]`)
	records, err := parseRecordsJSON(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Type != "A" {
		t.Errorf("expected type 'A', got %q", records[0].Type)
	}
	if records[0].Content != "1.2.3.4" {
		t.Errorf("expected content '1.2.3.4', got %q", records[0].Content)
	}
	if records[0].TTL != 300 {
		t.Errorf("expected TTL 300, got %d", records[0].TTL)
	}
	if *records[1].Priority != 10 {
		t.Errorf("expected priority 10, got %d", *records[1].Priority)
	}
}

func TestParseRecordsJSON_InvalidJSON(t *testing.T) {
	_, err := parseRecordsJSON([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseRecordsJSON_SkipsInvalidRows(t *testing.T) {
	// Row missing required type/name/content is skipped
	content := []byte(`[
		{"type":"A","name":"www","content":"1.2.3.4"},
		{"type":"","name":"","content":""},
		{"type":"MX","name":"@","content":"mail.example.com"}
	]`)
	records, err := parseRecordsJSON(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records (1 skipped), got %d", len(records))
	}
}

func TestParseRecordsCSV_Valid(t *testing.T) {
	content := []byte(`Type,Name,Content,TTL,Priority,Proxied,Comment
A,www,1.2.3.4,300,,false,test record
MX,@,mail.example.com,600,10,true,mail record`)
	records, err := parseRecordsCSV(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Type != "A" {
		t.Errorf("expected type 'A', got %q", records[0].Type)
	}
	if records[0].Content != "1.2.3.4" {
		t.Errorf("expected content '1.2.3.4', got %q", records[0].Content)
	}
	if records[0].TTL != 300 {
		t.Errorf("expected TTL 300, got %d", records[0].TTL)
	}
	if records[1].Type != "MX" {
		t.Errorf("expected type 'MX', got %q", records[1].Type)
	}
	if *records[1].Priority != 10 {
		t.Errorf("expected priority 10, got %d", *records[1].Priority)
	}
}

func TestParseRecordsCSV_DefaultTTL(t *testing.T) {
	content := []byte(`Type,Name,Content,TTL,Priority,Proxied
A,www,1.2.3.4,,,`)
	records, err := parseRecordsCSV(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if records[0].TTL != 300 {
		t.Errorf("expected default TTL 300, got %d", records[0].TTL)
	}
}

func TestParseRecordsCSV_InvalidCSV(t *testing.T) {
	_, err := parseRecordsCSV([]byte(`not,csv,at,all`))
	if err == nil {
		t.Error("expected error for invalid CSV")
	}
}

func TestParseRecordsCSV_TooFewRows(t *testing.T) {
	_, err := parseRecordsCSV([]byte(`Type,Name,Content`))
	if err == nil {
		t.Error("expected error for CSV with only header")
	}
}

func TestParseRecordsCSV_SkipsRowsWithMissingFields(t *testing.T) {
	// Row has all columns but empty required fields — should be skipped
	content := []byte(`Type,Name,Content,TTL,Priority,Proxied,Comment
A,www,1.2.3.4,300,,,valid record
,,,300,,,missing type/name/content`)

	records, err := parseRecordsCSV(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 valid record, got %d", len(records))
	}
}

func TestParseRecordsCSV_OptionalFields(t *testing.T) {
	content := []byte(`Type,Name,Content,TTL,Priority,Proxied,Comment
TXT,@,v=spf1 include:_spf.example.com ~all,3600,,,SPF record`)

	records, err := parseRecordsCSV(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Comment != "SPF record" {
		t.Errorf("expected comment 'SPF record', got %q", records[0].Comment)
	}
}

func TestExportRecordsJSON(t *testing.T) {
	priority := 10
	proxied := true
	records := []provider.DNSRecord{
		{ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4", TTL: 300, Proxied: &proxied, Comment: "test"},
		{ID: "r2", Type: "MX", Name: "@", Content: "mail.example.com", TTL: 600, Priority: &priority},
	}
	payload, _ := exportRecordsJSON(records)

	var output []map[string]any
	if err := json.Unmarshal(payload, &output); err != nil {
		t.Fatalf("exported JSON is not valid JSON: %v", err)
	}
	if len(output) != 2 {
		t.Fatalf("expected 2 records in JSON, got %d", len(output))
	}
	if output[0]["type"] != "A" {
		t.Errorf("expected type 'A', got %v", output[0]["type"])
	}
	if output[0]["content"] != "1.2.3.4" {
		t.Errorf("expected content '1.2.3.4', got %v", output[0]["content"])
	}
}

func TestExportRecordsCSV(t *testing.T) {
	records := []provider.DNSRecord{
		{ID: "r1", Type: "A", Name: "www", Content: "1.2.3.4", TTL: 300},
		{ID: "r2", Type: "MX", Name: "@", Content: "mail.example.com", TTL: 600},
	}
	payload := exportRecordsCSV(records)

	// Should contain header + 2 data rows
	lines := bytes.Split(payload, []byte("\n"))
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	if !bytes.Contains(lines[0], []byte("Type")) {
		t.Errorf("expected header to contain 'Type', got %s", lines[0])
	}
	if !bytes.Contains(lines[1], []byte("www")) {
		t.Errorf("expected first data row to contain 'www', got %s", lines[1])
	}
}

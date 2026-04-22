package service

import (
	"testing"

	"dns-hub/server/internal/provider"
)

func TestSummarizePropagation_AllMatched(t *testing.T) {
	status, summary := summarizePropagation(5, 0, 5)
	if status != "verified" {
		t.Errorf("expected verified, got %s", status)
	}
	if summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestSummarizePropagation_Partial(t *testing.T) {
	status, summary := summarizePropagation(2, 0, 5)
	if status != "partial" {
		t.Errorf("expected partial, got %s", status)
	}
	if summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestSummarizePropagation_Failed(t *testing.T) {
	status, _ := summarizePropagation(0, 5, 5)
	if status != "failed" {
		t.Errorf("expected failed, got %s", status)
	}
}

func TestSummarizePropagation_Pending(t *testing.T) {
	status, _ := summarizePropagation(0, 0, 5)
	if status != "pending" {
		t.Errorf("expected pending, got %s", status)
	}
}

func TestSummarizePropagation_ZeroResolvers(t *testing.T) {
	status, summary := summarizePropagation(0, 0, 0)
	if status != "unknown" {
		t.Errorf("expected unknown, got %s", status)
	}
	if summary == "" {
		t.Error("summary should not be empty for unknown status")
	}
}

func TestBuildFQDN(t *testing.T) {
	tests := []struct {
		name   string
		zone   string
		record string
		want   string
	}{
		{"simple", "example.com", "www", "www.example.com."},
		{"apex", "example.com", "@", "example.com."},
		{"already fqdn", "example.com", "www.example.com.", "www.example.com."},
		{"nested", "example.com", "blog.dev.example.com.", "blog.dev.example.com."},
		{"with trailing dot", "example.com.", "www", "www.example.com."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFQDN(tt.record, tt.zone)
			if got != tt.want {
				t.Errorf("buildFQDN(%q, %q) = %q, want %q", tt.record, tt.zone, got, tt.want)
			}
		})
	}
}

func TestAnswerMatches(t *testing.T) {
	content := "1.2.3.4"
	tests := []struct {
		name     string
		record   provider.DNSRecord
		answers  []string
		expected bool
	}{
		{
			name:     "exact match",
			record:   provider.DNSRecord{Content: content},
			answers:  []string{"1.2.3.4"},
			expected: true,
		},
		{
			name:     "partial match",
			record:   provider.DNSRecord{Content: content},
			answers:  []string{"some text 1.2.3.4 more text"},
			expected: true,
		},
		{
			name:     "no match",
			record:   provider.DNSRecord{Content: content},
			answers:  []string{"5.6.7.8"},
			expected: false,
		},
		{
			name:     "empty answers",
			record:   provider.DNSRecord{Content: content},
			answers:  []string{},
			expected: false,
		},
		{
			name:     "case insensitive",
			record:   provider.DNSRecord{Content: "EXAMPLE.COM"},
			answers:  []string{"example.com."},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := answerMatches(tt.record, tt.answers)
			if got != tt.expected {
				t.Errorf("answerMatches = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPropagationReason(t *testing.T) {
	if propagationReason("ok", true, []string{"1.2.3.4"}) != "matched" {
		t.Error("matched should return 'matched'")
	}
	if propagationReason("ok", false, []string{}) != "no_answer" {
		t.Error("no answers should return 'no_answer'")
	}
	if propagationReason("ok", false, []string{"5.6.7.8"}) != "value_mismatch" {
		t.Error("mismatch should return 'value_mismatch'")
	}
	if propagationReason("timeout", false, nil) != "resolver_error" {
		t.Error("error status should return 'resolver_error'")
	}
}

func TestPropagationReason_Unknown(t *testing.T) {
	if propagationReason("unknown", false, nil) != "resolver_error" {
		t.Error("unknown status should return 'resolver_error'")
	}
}

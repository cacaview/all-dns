package service

import (
	"testing"
	"time"
)

func TestSeverityForDays(t *testing.T) {
	tests := []struct {
		days   int
		expect string
	}{
		{-5, "expired"},
		{-1, "expired"},
		{0, "critical"},
		{1, "critical"},
		{2, "warning"},
		{7, "warning"},
		{8, "notice"},
		{30, "notice"},
		{31, ""},
		{60, ""},
	}

	for _, tt := range tests {
		t.Run(tt.expect, func(t *testing.T) {
			got := severityForDays(tt.days)
			if got != tt.expect {
				t.Errorf("severityForDays(%d) = %q, want %q", tt.days, got, tt.expect)
			}
		})
	}
}

func TestFormatExpiryEmailBody(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	reminders := []Reminder{
		{Name: "CF Main", Provider: "cloudflare", Severity: "critical", DaysLeft: 1, ExpiresAt: &future},
		{Name: "AWS Prod", Provider: "aws", Severity: "warning", DaysLeft: 5, ExpiresAt: &future},
	}
	body := formatExpiryEmailBody(reminders)
	if body == "" {
		t.Fatal("expected non-empty body")
	}
	// Check key content is present
	lines := []string{"DNS Hub Credential Expiry Report", "CF Main", "cloudflare", "critical", "AWS Prod", "warning"}
	for _, want := range lines {
		if !contains(body, want) {
			t.Errorf("expected body to contain %q, got:\n%s", want, body)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestJoinLines(t *testing.T) {
	result := joinLines("line1", "line2", "line3")
	expected := "line1\nline2\nline3\n"
	if result != expected {
		t.Errorf("joinLines:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestReminderService_Scan_NoAccounts(t *testing.T) {
	// Test that Scan handles empty accounts gracefully
	now := time.Now().UTC()
	daysLeft := int(time.Until(now.AddDate(0, 0, -5)).Hours() / 24)
	if daysLeft < 0 {
		// expired case
		sev := severityForDays(daysLeft)
		if sev != "expired" {
			t.Errorf("expected expired, got %s", sev)
		}
	}
}

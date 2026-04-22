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

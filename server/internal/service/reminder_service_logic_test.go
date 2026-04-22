package service

import (
	"testing"
	"time"
)

func TestReminder_SeverityLevels(t *testing.T) {
	// expired: daysLeft < 0
	if s := severityForDays(-1); s != "expired" {
		t.Errorf("days=-1 → expired, got %s", s)
	}

	// critical: 0 <= daysLeft <= 1
	if s := severityForDays(0); s != "critical" {
		t.Errorf("days=0 → critical, got %s", s)
	}
	if s := severityForDays(1); s != "critical" {
		t.Errorf("days=1 → critical, got %s", s)
	}

	// warning: 2 <= daysLeft <= 7
	if s := severityForDays(2); s != "warning" {
		t.Errorf("days=2 → warning, got %s", s)
	}
	if s := severityForDays(7); s != "warning" {
		t.Errorf("days=7 → warning, got %s", s)
	}

	// notice: 8 <= daysLeft <= 30
	if s := severityForDays(8); s != "notice" {
		t.Errorf("days=8 → notice, got %s", s)
	}
	if s := severityForDays(30); s != "notice" {
		t.Errorf("days=30 → notice, got %s", s)
	}

	// no reminder: daysLeft > 30
	if s := severityForDays(31); s != "" {
		t.Errorf("days=31 → empty, got %s", s)
	}
}

func TestReminder_DaysLeftCalculation(t *testing.T) {
	now := time.Now().UTC()
	expiresAt := now.AddDate(0, 0, 5) // 5 days from now

	hours := expiresAt.Sub(now).Hours()
	daysLeft := int(hours / 24)

	if daysLeft != 5 {
		t.Errorf("expected 5 days left, got %d", daysLeft)
	}
}

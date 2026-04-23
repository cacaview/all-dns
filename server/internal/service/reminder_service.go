package service

import (
	"context"
	"fmt"
	"time"

	"dns-hub/server/internal/model"
	"dns-hub/server/internal/notifier"
	"gorm.io/gorm"
)

type Reminder struct {
	AccountID  uint       `json:"accountId"`
	Name       string     `json:"name"`
	Provider   string     `json:"provider"`
	UserID     uint       `json:"userId"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	Severity   string     `json:"severity"`
	DaysLeft   int        `json:"daysLeft"`
	Handled    bool       `json:"handled"`
	HandledAt  string     `json:"handledAt,omitempty"`
}

type ReminderService struct {
	db            *gorm.DB
	webhookSvc    *WebhookService
	emailNotifier *notifier.EmailNotifier
	dnsSvc        *DNSService
}

func NewReminderService(db *gorm.DB, webhookSvc *WebhookService, emailNotifier *notifier.EmailNotifier) *ReminderService {
	return &ReminderService{db: db, webhookSvc: webhookSvc, emailNotifier: emailNotifier}
}

// SetDNSService breaks the constructor-cycle with DNSService.
func (s *ReminderService) SetDNSService(dnsSvc *DNSService) {
	s.dnsSvc = dnsSvc
}

func (s *ReminderService) Scan(ctx context.Context) ([]Reminder, error) {
	var accounts []model.Account
	if err := s.db.WithContext(ctx).Where("expires_at IS NOT NULL").Find(&accounts).Error; err != nil {
		return nil, err
	}
	reminders := make([]Reminder, 0)
	now := time.Now().UTC()
	byOrg := make(map[uint][]Reminder)
	for _, account := range accounts {
		if account.ExpiresAt == nil {
			continue
		}
		daysLeft := int(account.ExpiresAt.Sub(now).Hours() / 24)
		severity := severityForDays(daysLeft)
		if severity == "" {
			continue
		}
		reminder := Reminder{
			AccountID: account.ID,
			Name:      account.Name,
			Provider:  account.Provider,
			UserID:    account.UserID,
			ExpiresAt: account.ExpiresAt,
			Severity:  severity,
			DaysLeft:  daysLeft,
		}
		reminders = append(reminders, reminder)
		byOrg[account.OrgID] = append(byOrg[account.OrgID], reminder)
	}
for orgID, orgReminders := range byOrg {
		payload := map[string]any{
			"type":        "credential_expiry",
			"generatedAt": now.Format(time.RFC3339),
			"reminders":   orgReminders,
		}
		_ = s.webhookSvc.NotifyAll(ctx, orgID, "credential_expiry", payload)
		if s.emailNotifier != nil && s.emailNotifier.Enabled() {
			_ = s.sendExpiryEmails(ctx, orgID, orgReminders)
		}
		// Auto-reactivate accounts that were in error state — credentials may have been renewed manually
		if s.dnsSvc != nil {
			for _, reminder := range orgReminders {
				if reminder.Severity == "expired" || reminder.Severity == "critical" {
					_ = s.dnsSvc.ReactivateAccount(ctx, reminder.UserID, reminder.AccountID)
				}
			}
		}
	}
	return reminders, nil
}

func (s *ReminderService) sendExpiryEmails(ctx context.Context, orgID uint, reminders []Reminder) error {
	org, err := s.getOrgUsers(ctx, orgID)
	if err != nil || len(org.users) == 0 {
		return err
	}

	subject := fmt.Sprintf("DNS Hub: %d credential(s) expiring soon", len(reminders))
	body := formatExpiryEmailBody(reminders)

	payload := map[string]any{
		"to":      org.users,
		"subject": subject,
		"body":    body,
	}
	return s.emailNotifier.Notify(ctx, payload)
}

func formatExpiryEmailBody(reminders []Reminder) string {
	var lines []string
	lines = append(lines, "DNS Hub Credential Expiry Report")
	lines = append(lines, "==================================")
	lines = append(lines, "")
	for _, r := range reminders {
		lines = append(lines, fmt.Sprintf("- [%s] %s (%s): %s (%d days left)",
			r.Severity, r.Name, r.Provider, r.ExpiresAt.Format("2006-01-02"), r.DaysLeft))
	}
	lines = append(lines, "")
	lines = append(lines, "Visit DNS Hub to review and update credentials before they expire.")
	return fmt.Sprintf("%s\n", joinLines(lines...))
}

func joinLines(parts ...string) string {
	result := ""
	for _, p := range parts {
		result += p + "\n"
	}
	return result
}

type orgUserSet struct {
	users []string
}

func (s *ReminderService) getOrgUsers(ctx context.Context, orgID uint) (*orgUserSet, error) {
	var users []model.User
	if err := s.db.WithContext(ctx).Where("primary_org_id = ?", orgID).Find(&users).Error; err != nil {
		return nil, err
	}
	set := &orgUserSet{}
	for _, u := range users {
		if u.Email != "" {
			set.users = append(set.users, u.Email)
		}
	}
	return set, nil
}

func (s *ReminderService) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.Scan(ctx)
			}
		}
	}()
}

// ReminderAck tracks whether a user has handled a reminder for a specific account.
type ReminderAck struct {
	AccountID uint   `json:"accountId"`
	UserID    uint   `json:"userId"`
	Handled   bool   `json:"handled"`
	HandledAt string `json:"handledAt,omitempty"`
}

func (s *ReminderService) GetReminderAcks(userID uint) (map[uint]ReminderAck, error) {
	var acks []model.ReminderAck
	if err := s.db.Where("user_id = ?", userID).Find(&acks).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]ReminderAck, len(acks))
	for _, ack := range acks {
		result[ack.AccountID] = ReminderAck{
			AccountID: ack.AccountID,
			UserID:    ack.UserID,
			Handled:   true,
			HandledAt: ack.HandledAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return result, nil
}

func (s *ReminderService) SetReminderHandled(userID, accountID uint, handled bool) error {
	if handled {
		ack := model.ReminderAck{
			UserID:    userID,
			AccountID: accountID,
			HandledAt: time.Now().UTC(),
		}
		return s.db.Where("user_id = ? AND account_id = ?", userID, accountID).
			Assign(ack).FirstOrCreate(&ack).Error
	}
	return s.db.Where("user_id = ? AND account_id = ?", userID, accountID).Delete(&model.ReminderAck{}).Error
}

func severityForDays(daysLeft int) string {
	switch {
	case daysLeft < 0:
		return "expired"
	case daysLeft <= 1:
		return "critical"
	case daysLeft <= 7:
		return "warning"
	case daysLeft <= 30:
		return "notice"
	default:
		return ""
	}
}

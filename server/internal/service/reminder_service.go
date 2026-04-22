package service

import (
	"context"
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
	db       *gorm.DB
	notifier *notifier.WebhookNotifier
}

func NewReminderService(db *gorm.DB, webhook *notifier.WebhookNotifier) *ReminderService {
	return &ReminderService{db: db, notifier: webhook}
}

func (s *ReminderService) Scan(ctx context.Context) ([]Reminder, error) {
	var accounts []model.Account
	if err := s.db.WithContext(ctx).Where("expires_at IS NOT NULL").Find(&accounts).Error; err != nil {
		return nil, err
	}
	reminders := make([]Reminder, 0)
	now := time.Now().UTC()
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
	}
	if len(reminders) > 0 {
		_ = s.notifier.Notify(ctx, map[string]any{
			"type":      "credential_expiry",
			"generatedAt": now.Format(time.RFC3339),
			"reminders": reminders,
		})
	}
	return reminders, nil
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

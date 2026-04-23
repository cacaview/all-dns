package service

import (
	"context"
	"strings"

	"dns-hub/server/internal/model"
	"dns-hub/server/internal/notifier"
	"gorm.io/gorm"
)

// WebhookService handles Webhook CRUD and notification delivery.
type WebhookService struct {
	db       *gorm.DB
	notifier *notifier.WebhookNotifier
}

// NewWebhookService creates a WebhookService.
func NewWebhookService(db *gorm.DB, webhookNotifier *notifier.WebhookNotifier) *WebhookService {
	return &WebhookService{db: db, notifier: webhookNotifier}
}

// ListWebhooks returns all webhooks for the given org.
func (s *WebhookService) ListWebhooks(orgID uint) ([]model.Webhook, error) {
	var webhooks []model.Webhook
	if err := s.db.Where("org_id = ?", orgID).Order("created_at desc").Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}

// GetWebhook returns a single webhook if it belongs to the org.
func (s *WebhookService) GetWebhook(orgID, webhookID uint) (*model.Webhook, error) {
	var webhook model.Webhook
	if err := s.db.Where("id = ? AND org_id = ?", webhookID, orgID).First(&webhook).Error; err != nil {
		return nil, err
	}
	return &webhook, nil
}

// CreateWebhook creates a new webhook.
func (s *WebhookService) CreateWebhook(orgID uint, name, url string, events []string) (*model.Webhook, error) {
	webhook := &model.Webhook{
		OrgID:  orgID,
		Name:   strings.TrimSpace(name),
		URL:    strings.TrimSpace(url),
		Events: toWebhookEvents(events),
		Active: true,
	}
	if err := s.db.Create(webhook).Error; err != nil {
		return nil, err
	}
	return webhook, nil
}

// UpdateWebhook updates name, URL, events, and active status.
func (s *WebhookService) UpdateWebhook(orgID, webhookID uint, name, url string, events []string, active bool) (*model.Webhook, error) {
	webhook, err := s.GetWebhook(orgID, webhookID)
	if err != nil {
		return nil, err
	}
	if name != "" {
		webhook.Name = strings.TrimSpace(name)
	}
	if url != "" {
		webhook.URL = strings.TrimSpace(url)
	}
	webhook.Events = toWebhookEvents(events)
	webhook.Active = active
	if err := s.db.Save(webhook).Error; err != nil {
		return nil, err
	}
	return webhook, nil
}

// DeleteWebhook deletes a webhook.
func (s *WebhookService) DeleteWebhook(orgID, webhookID uint) error {
	result := s.db.Where("id = ? AND org_id = ?", webhookID, orgID).Delete(&model.Webhook{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// NotifyAll sends a payload to all active webhooks for the org that are subscribed to the given event.
func (s *WebhookService) NotifyAll(ctx context.Context, orgID uint, event string, payload any) error {
	var webhooks []model.Webhook
	if err := s.db.Where("org_id = ? AND active = ?", orgID, true).Find(&webhooks).Error; err != nil {
		return err
	}
	for _, wh := range webhooks {
		if !subscribesTo(wh.Events, event) {
			continue
		}
		if wh.URL == "" {
			continue
		}
		n := notifier.NewWebhookNotifier(wh.URL)
		_ = n.Notify(ctx, payload)
	}
	return nil
}

func toWebhookEvents(events []string) map[string]any {
	items := make([]any, 0, len(events))
	for _, e := range events {
		trimmed := strings.TrimSpace(e)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return map[string]any{"events": items}
}

func subscribesTo(events map[string]any, event string) bool {
	raw, ok := events["events"]
	if !ok {
		return false
	}
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, e := range items {
		if s, ok := e.(string); ok && s == event {
			return true
		}
	}
	return false
}

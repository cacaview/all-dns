package model

import (
	"time"

	"gorm.io/datatypes"
)

// Webhook stores a per-org webhook endpoint configuration.
type Webhook struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	OrgID     uint           `gorm:"uniqueIndex:idx_webhook_org_name;not null" json:"orgId"`
	Name      string         `gorm:"size=255;not null" json:"name"`
	URL       string         `gorm:"size=2048;not null" json:"url"`
	Events    datatypes.JSONMap `gorm:"type:jsonb;not null" json:"events"` // e.g. {"events": ["credential_expiry"]}
	Active    bool           `gorm:"not null;default:true" json:"active"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// WebhookView is the API response shape.
type WebhookView struct {
	ID        uint     `json:"id"`
	OrgID     uint     `json:"orgId"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Active    bool     `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ToView converts a Webhook to its API view.
func (w *Webhook) ToView() WebhookView {
	events := []string{}
	if raw, ok := w.Events["events"].([]any); ok {
		for _, e := range raw {
			if s, ok := e.(string); ok {
				events = append(events, s)
			}
		}
	} else if arr, ok := w.Events["events"].([]interface{}); ok {
		// Handle []interface{} from some JSON drivers
		for _, e := range arr {
			if s, ok := e.(string); ok {
				events = append(events, s)
			}
		}
	}
	return WebhookView{
		ID:        w.ID,
		OrgID:     w.OrgID,
		Name:      w.Name,
		URL:       w.URL,
		Events:    events,
		Active:    w.Active,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}
}

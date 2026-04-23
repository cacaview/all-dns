package model

import (
	"time"
)

// AuditAction records a user action for audit purposes.
type AuditLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"userId"`
	OrgID        uint      `gorm:"index;not null" json:"orgId"`
	Action       string    `gorm:"size=64;not null" json:"action"`
	Resource     string    `gorm:"size=64;not null" json:"resource"`       // e.g. "domain", "account", "webhook"
	ResourceID   uint      `gorm:"index" json:"resourceId"`                // ID of the affected resource
	Description  string    `gorm:"size=500" json:"description"`            // Human-readable description
	IPAddress    string    `gorm:"size=45" json:"ipAddress"`              // IPv4 or IPv6
	UserAgent    string    `gorm:"size=500" json:"userAgent"`              // Browser/client user agent
	RequestPath  string    `gorm:"size=2048" json:"requestPath"`           // Full request path
	RequestMethod string   `gorm:"size=10" json:"requestMethod"`           // GET, POST, etc.
	StatusCode   int       `gorm:"default:0" json:"statusCode"`            // HTTP response status
	CreatedAt    time.Time `json:"createdAt"`
}

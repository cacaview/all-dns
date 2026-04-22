package model

import (
	"time"

	"gorm.io/datatypes"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

type User struct {
	ID            uint              `gorm:"primaryKey" json:"id"`
	Email         string            `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Role          Role              `gorm:"size:32;not null;default:viewer" json:"role"`
	PrimaryOrgID  uint              `gorm:"index;default:0" json:"primaryOrgId"`
	OAuthProvider string            `gorm:"column:oauth_provider;size:32;not null" json:"oauthProvider"`
	OAuthSubject  string            `gorm:"column:oauth_subject;size:255;not null;index" json:"oauthSubject"`
	OAuthInfo     datatypes.JSONMap `gorm:"column:oauth_info;type:jsonb;not null" json:"oauthInfo"`
	TokenVersion  int               `gorm:"column:token_version;not null;default:1" json:"tokenVersion"`
	CreatedAt     time.Time         `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time         `gorm:"column:updated_at" json:"updatedAt"`
}

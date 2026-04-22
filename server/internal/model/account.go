package model

import (
	"time"

	"gorm.io/datatypes"
)

type Account struct {
	ID                  uint              `gorm:"primaryKey" json:"id"`
	OrgID               uint              `gorm:"index;not null" json:"orgId"`
	UserID              uint              `gorm:"index;not null" json:"userId"` // creator/owner within the org
	Name                string            `gorm:"size:255;not null" json:"name"`
	Provider            string            `gorm:"size:64;not null;index" json:"provider"`
	EncryptedConfig     datatypes.JSONMap `gorm:"type:jsonb;not null" json:"-"`
	ExpiresAt           *time.Time        `json:"expiresAt"`
	LastCheckedAt       *time.Time        `json:"lastCheckedAt"`
	LastRotatedAt       *time.Time        `json:"lastRotatedAt"`
	LastValidationError string            `gorm:"type:text;not null;default:''" json:"lastValidationError"`
	CredentialStatus    string            `gorm:"size:32;not null;default:unknown" json:"credentialStatus"`
	Status              string            `gorm:"size:32;not null;default:active" json:"status"`
	CreatedAt           time.Time         `json:"createdAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
}

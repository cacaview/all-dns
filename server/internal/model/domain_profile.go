package model

import (
	"time"

	"gorm.io/datatypes"
)

type DomainProfile struct {
	ID             uint              `gorm:"primaryKey" json:"id"`
	DomainID       uint              `gorm:"uniqueIndex;not null" json:"domainId"`
	Description    string            `gorm:"type:text;not null;default:''" json:"description"`
	AttachmentURLs datatypes.JSONMap `gorm:"type:jsonb;not null" json:"attachmentUrls"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

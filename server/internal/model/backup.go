package model

import (
	"time"

	"gorm.io/datatypes"
)

type Backup struct {
	ID                uint              `gorm:"primaryKey" json:"id"`
	DomainID          uint              `gorm:"index;not null" json:"domainId"`
	TriggeredByUserID uint              `gorm:"index;not null" json:"triggeredByUserId"`
	Reason            string            `gorm:"size:255;not null" json:"reason"`
	Content           datatypes.JSONMap `gorm:"type:jsonb;not null" json:"content"`
	CreatedAt         time.Time         `json:"createdAt"`
}

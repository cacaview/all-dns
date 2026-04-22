package model

import (
	"time"

	"gorm.io/datatypes"
)

type Domain struct {
	ID                    uint              `gorm:"primaryKey" json:"id"`
	AccountID             uint              `gorm:"index;not null" json:"accountId"`
	Name                  string            `gorm:"size:255;not null;index" json:"name"`
	ProviderZoneID        string            `gorm:"size:255;not null;index" json:"providerZoneId"`
	IsStarred             bool              `gorm:"not null;default:false" json:"isStarred"`
	IsArchived            bool              `gorm:"not null;default:false" json:"isArchived"`
	ArchivedAt            *time.Time        `json:"archivedAt"`
	Tags                  datatypes.JSONMap `gorm:"type:jsonb;not null" json:"tags"`
	LastSyncedAt          *time.Time        `json:"lastSyncedAt"`
	LastPropagationStatus datatypes.JSONMap `gorm:"type:jsonb;not null" json:"lastPropagationStatus"`
	CreatedAt             time.Time         `json:"createdAt"`
	UpdatedAt             time.Time         `json:"updatedAt"`
}

type PropagationCheck struct {
	ID                uint              `gorm:"primaryKey" json:"id"`
	DomainID          uint              `gorm:"index;not null" json:"domainId"`
	TriggeredByUserID uint              `gorm:"index;not null" json:"triggeredByUserId"`
	FQDN              string            `gorm:"size:255;not null" json:"fqdn"`
	Record            datatypes.JSONMap `gorm:"type:jsonb;not null" json:"record"`
	OverallStatus     string            `gorm:"size:32;not null" json:"overallStatus"`
	Summary           string            `gorm:"size:255;not null" json:"summary"`
	MatchedCount      int               `gorm:"not null;default:0" json:"matchedCount"`
	FailedCount       int               `gorm:"not null;default:0" json:"failedCount"`
	PendingCount      int               `gorm:"not null;default:0" json:"pendingCount"`
	TotalResolvers    int               `gorm:"not null;default:0" json:"totalResolvers"`
	Results           datatypes.JSONMap `gorm:"type:jsonb;not null" json:"results"`
	CheckedAt         time.Time         `gorm:"index;not null" json:"checkedAt"`
	CreatedAt         time.Time         `json:"createdAt"`
}

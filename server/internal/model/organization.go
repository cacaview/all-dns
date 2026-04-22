package model

import (
	"time"
)

// Organization represents a tenant / workspace that owns DNS accounts and domains.
type Organization struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// OrgMember links a user to an organization with a specific role.
type OrgMember struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	OrganizationID uint      `gorm("uniqueIndex:idx_org_user;not null")`
	UserID         uint      `gorm("uniqueIndex:idx_org_user;not null")`
	Role           Role      `gorm:"size:32;not null;default:viewer" json:"role"`
	CreatedAt      time.Time `json:"createdAt"`
}

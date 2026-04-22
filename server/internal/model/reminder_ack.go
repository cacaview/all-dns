package model

import (
	"time"
)

// ReminderAck tracks whether a user has marked a credential expiry reminder as handled.
type ReminderAck struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_account;not null" json:"userId"`
	AccountID uint      `gorm:"uniqueIndex:idx_user_account;not null" json:"accountId"`
	HandledAt time.Time `gorm:"not null" json:"handledAt"`
	CreatedAt time.Time `json:"createdAt"`
}

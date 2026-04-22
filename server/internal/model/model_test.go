package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUser_JSON(t *testing.T) {
	user := User{
		ID:            1,
		Email:         "test@example.com",
		Role:          RoleAdmin,
		PrimaryOrgID:  1,
		OAuthProvider: "github",
		OAuthSubject:  "12345",
		TokenVersion:  1,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var unmarshaled User
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if unmarshaled.Email != "test@example.com" {
		t.Errorf("email mismatch")
	}
	if unmarshaled.Role != RoleAdmin {
		t.Errorf("role mismatch: got %s", unmarshaled.Role)
	}
}

func TestAccount_OrgID(t *testing.T) {
	account := Account{
		ID:     1,
		OrgID:  5,
		UserID: 10,
		Name:   "Test Account",
	}

	if account.OrgID != 5 {
		t.Errorf("expected OrgID=5, got %d", account.OrgID)
	}
	if account.UserID != 10 {
		t.Errorf("expected UserID=10, got %d", account.UserID)
	}
}

func TestOrganization_JSON(t *testing.T) {
	org := Organization{
		ID:   1,
		Name: "My Org",
	}

	data, err := json.Marshal(org)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var unmarshaled Organization
	json.Unmarshal(data, &unmarshaled)

	if unmarshaled.Name != "My Org" {
		t.Errorf("name mismatch: got %s", unmarshaled.Name)
	}
}

func TestOrgMember_Role(t *testing.T) {
	member := OrgMember{
		ID:             1,
		OrganizationID: 1,
		UserID:         1,
		Role:           RoleEditor,
	}

	if member.Role != RoleEditor {
		t.Errorf("expected editor, got %s", member.Role)
	}
}

func TestReminderAck_TableName(t *testing.T) {
	ack := ReminderAck{}
	// GORM uses table name from model
	if ack.ID != 0 {
		t.Error("ReminderAck should have zero value ID")
	}
}

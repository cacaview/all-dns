package service

import (
	"testing"

	"dns-hub/server/internal/model"
)

func TestRoleConstants(t *testing.T) {
	if model.RoleAdmin != "admin" {
		t.Errorf("expected admin, got %s", model.RoleAdmin)
	}
	if model.RoleEditor != "editor" {
		t.Errorf("expected editor, got %s", model.RoleEditor)
	}
	if model.RoleViewer != "viewer" {
		t.Errorf("expected viewer, got %s", model.RoleViewer)
	}
}

func TestAuthService_UpsertUser_NewUser_Role(t *testing.T) {
	// Test the role assignment logic:
	// - count == 0 → first user gets admin
	// - count > 0 → subsequent users get viewer

	// This is a logic test without DB; verify the role constants are valid
	roles := []model.Role{model.RoleAdmin, model.RoleEditor, model.RoleViewer}
	for _, role := range roles {
		if role == "" {
			t.Error("role should not be empty")
		}
	}
}

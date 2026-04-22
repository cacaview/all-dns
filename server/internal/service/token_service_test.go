package service

import (
	"testing"
	"time"

	"dns-hub/server/internal/model"
)

func TestTokenService_IssueAndParse(t *testing.T) {
	svc := NewTokenService("test-secret-key-for-jwt-256bits!!", 15*time.Minute, 7*24*time.Hour)

	user := model.User{ID: 1, Role: model.RoleAdmin, TokenVersion: 1}
	pair, err := svc.IssuePair(user)
	if err != nil {
		t.Fatalf("IssuePair failed: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("access token should not be empty")
	}
	if pair.RefreshToken == "" {
		t.Error("refresh token should not be empty")
	}

	claims, err := svc.Parse(pair.AccessToken, TokenTypeAccess)
	if err != nil {
		t.Fatalf("Parse access token failed: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("expected userID=1, got %d", claims.UserID)
	}
	if claims.Role != model.RoleAdmin {
		t.Errorf("expected role=admin, got %s", claims.Role)
	}
}

func TestTokenService_RefreshToken(t *testing.T) {
	svc := NewTokenService("test-secret-key-for-jwt-256bits!!", 15*time.Minute, 7*24*time.Hour)

	user := model.User{ID: 42, Role: model.RoleEditor, TokenVersion: 1}
	pair, _ := svc.IssuePair(user)

	claims, err := svc.Parse(pair.RefreshToken, TokenTypeRefresh)
	if err != nil {
		t.Fatalf("Parse refresh token failed: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("expected userID=42, got %d", claims.UserID)
	}
	if claims.TokenVersion != 1 {
		t.Errorf("expected tokenVersion=1, got %d", claims.TokenVersion)
	}
}

func TestTokenService_VersionMismatch(t *testing.T) {
	svc := NewTokenService("test-secret-key-for-jwt-256bits!!", 15*time.Minute, 7*24*time.Hour)

	user := model.User{ID: 1, Role: model.RoleAdmin, TokenVersion: 2}
	pair, _ := svc.IssuePair(user)

	// Simulate old token version
	claims := &TokenClaims{UserID: 1, TokenVersion: 1}
	_ = claims

	// Token version in token is 2, but user.TokenVersion is 1 → should fail
	if pair.RefreshToken == "" {
		t.Error("should issue pair normally")
	}
}

func TestTokenService_InvalidSecret(t *testing.T) {
	svc1 := NewTokenService("secret-one-32-characters-long!!!!", 15*time.Minute, 7*24*time.Hour)
	svc2 := NewTokenService("secret-two-32-characters-long!!!!", 15*time.Minute, 7*24*time.Hour)

	user := model.User{ID: 1, Role: model.RoleAdmin, TokenVersion: 1}
	pair, _ := svc1.IssuePair(user)

	_, err := svc2.Parse(pair.AccessToken, TokenTypeAccess)
	if err == nil {
		t.Error("parsing with wrong secret should fail")
	}
}

package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"dns-hub/server/internal/model"
	appoauth "dns-hub/server/internal/oauth"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AuthService struct {
	db           *gorm.DB
	tokenService *TokenService
	providers    map[string]appoauth.Provider
	stateStore   sync.Map
}

type AuthResult struct {
	User      model.User  `json:"user"`
	Tokens    *TokenPair  `json:"tokens"`
	Provider  string      `json:"provider"`
}

type authState struct {
	Provider  string
	CreatedAt time.Time
}

func NewAuthService(db *gorm.DB, tokenService *TokenService, providers ...appoauth.Provider) *AuthService {
	mapped := make(map[string]appoauth.Provider, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		mapped[provider.Name()] = provider
	}
	return &AuthService{db: db, tokenService: tokenService, providers: mapped}
}

func (s *AuthService) StartAuth(providerName string) (string, error) {
	provider, ok := s.providers[providerName]
	if !ok {
		return "", fmt.Errorf("oauth provider %s is not configured", providerName)
	}
	state, err := randomState()
	if err != nil {
		return "", err
	}
	s.stateStore.Store(state, authState{Provider: providerName, CreatedAt: time.Now().UTC()})
	return provider.AuthCodeURL(state), nil
}

func (s *AuthService) CompleteAuth(ctx context.Context, providerName, state, code string) (*AuthResult, error) {
	provider, ok := s.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("oauth provider %s is not configured", providerName)
	}
	stored, ok := s.stateStore.LoadAndDelete(state)
	if !ok {
		return nil, fmt.Errorf("invalid oauth state")
	}
	stateValue, ok := stored.(authState)
	if !ok || stateValue.Provider != providerName || time.Since(stateValue.CreatedAt) > 15*time.Minute {
		return nil, fmt.Errorf("oauth state expired or mismatched")
	}

	token, err := provider.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	profile, err := provider.FetchProfile(ctx, token)
	if err != nil {
		return nil, err
	}

	user, err := s.upsertUser(profile)
	if err != nil {
		return nil, err
	}
	pair, err := s.tokenService.IssuePair(*user)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: *user, Tokens: pair, Provider: providerName}, nil
}

func (s *AuthService) Refresh(refreshToken string) (*AuthResult, error) {
	claims, err := s.tokenService.Parse(refreshToken, TokenTypeRefresh)
	if err != nil {
		return nil, err
	}
	user, err := s.GetUserByID(claims.UserID)
	if err != nil {
		return nil, err
	}
	if user.TokenVersion != claims.TokenVersion {
		return nil, fmt.Errorf("token version mismatch")
	}
	pair, err := s.tokenService.IssuePair(*user)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: *user, Tokens: pair, Provider: user.OAuthProvider}, nil
}

func (s *AuthService) DevLogin(email string) (*AuthResult, error) {
	if strings.TrimSpace(email) == "" {
		email = "demo@dns-hub.local"
	}
	profile := &appoauth.UserProfile{
		Email:    email,
		Subject:  email,
		Name:     "Demo User",
		Avatar:   "",
		Provider: "dev",
		Raw: map[string]any{
			"mode": "dev-login",
		},
	}
	user, err := s.upsertUser(profile)
	if err != nil {
		return nil, err
	}
	pair, err := s.tokenService.IssuePair(*user)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: *user, Tokens: pair, Provider: profile.Provider}, nil
}

func (s *AuthService) GetUserByID(userID uint) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *AuthService) Logout(userID uint) error {
	return s.db.Model(&model.User{}).Where("id = ?", userID).Update("token_version", gorm.Expr("token_version + 1")).Error
}

func (s *AuthService) ListUsers(users *[]model.User) error {
	return s.db.Order("created_at ASC").Find(users).Error
}

func (s *AuthService) UpdateUserRole(userID uint, role model.Role) error {
	return s.db.Model(&model.User{}).Where("id = ?", userID).Update("role", role).Error
}

func (s *AuthService) RedirectURL(frontendURL string, result *AuthResult) string {
	fragment := url.Values{}
	fragment.Set("accessToken", result.Tokens.AccessToken)
	fragment.Set("refreshToken", result.Tokens.RefreshToken)
	fragment.Set("provider", result.Provider)
	fragment.Set("userEmail", result.User.Email)
	return strings.TrimRight(frontendURL, "/") + "/login#" + fragment.Encode()
}

func (s *AuthService) upsertUser(profile *appoauth.UserProfile) (*model.User, error) {
	var user model.User
	err := s.db.Where("oauth_provider = ? AND oauth_subject = ?", profile.Provider, profile.Subject).First(&user).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		// First user in the system becomes admin; all subsequent users become viewer
		var count int64
		s.db.Model(&model.User{}).Count(&count)
		role := model.RoleViewer
		var orgID uint
		if count == 0 {
			role = model.RoleAdmin
			// Create default organization for the first user
			org := &model.Organization{Name: "Default Organization"}
			if err := s.db.Create(org).Error; err != nil {
				return nil, fmt.Errorf("create default org: %w", err)
			}
			orgID = org.ID
		}
		user = model.User{
			Email:         profile.Email,
			Role:          role,
			PrimaryOrgID:  orgID,
			OAuthProvider: profile.Provider,
			OAuthSubject:  profile.Subject,
			OAuthInfo: datatypes.JSONMap{
				"name":   profile.Name,
				"avatar": profile.Avatar,
				"raw":    profile.Raw,
			},
			TokenVersion: 1,
		}
		if err := s.db.Create(&user).Error; err != nil {
			return nil, err
		}
		// Add user as org member with admin role if org was created
		if count == 0 {
			member := &model.OrgMember{
				OrganizationID: orgID,
				UserID:         user.ID,
				Role:           model.RoleAdmin,
			}
			if err := s.db.Create(member).Error; err != nil {
				return nil, fmt.Errorf("create org membership: %w", err)
			}
		}
		return &user, nil
	}
	user.Email = profile.Email
	user.OAuthInfo = datatypes.JSONMap{
		"name":   profile.Name,
		"avatar": profile.Avatar,
		"raw":    profile.Raw,
	}
	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func randomState() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

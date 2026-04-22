package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"dns-hub/server/internal/config"
	golangoauth "golang.org/x/oauth2"
	"golang.org/x/oauth2/gitlab"
)

type GitLabProvider struct {
	config *golangoauth.Config
}

type gitLabUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar_url"`
	Email    string `json:"email"`
}

func NewGitLabProvider(cfg config.Config) *GitLabProvider {
	return &GitLabProvider{config: &golangoauth.Config{
		ClientID:     cfg.GitLabClientID,
		ClientSecret: cfg.GitLabClientSecret,
		RedirectURL:  cfg.GitLabRedirectURL,
		Endpoint:     gitlab.Endpoint,
		Scopes:       []string{"read_user", "openid", "profile", "email"},
	}}
}

func (p *GitLabProvider) Name() string {
	return "gitlab"
}

func (p *GitLabProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *GitLabProvider) Exchange(ctx context.Context, code string) (*golangoauth.Token, error) {
	return p.config.Exchange(ctx, code)
}

func (p *GitLabProvider) FetchProfile(ctx context.Context, token *golangoauth.Token) (*UserProfile, error) {
	client := p.config.Client(ctx, token)
	request, err := http.NewRequest(http.MethodGet, "https://gitlab.com/api/v4/user", nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch gitlab user: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch gitlab user failed: %s", strings.TrimSpace(string(payload)))
	}
	var user gitLabUser
	if err := json.Unmarshal(payload, &user); err != nil {
		return nil, err
	}
	if strings.TrimSpace(user.Email) == "" {
		return nil, fmt.Errorf("gitlab account did not provide an email")
	}

	return &UserProfile{
		Email:    strings.ToLower(strings.TrimSpace(user.Email)),
		Subject:  fmt.Sprintf("%d", user.ID),
		Name:     firstNonEmpty(user.Name, user.Username),
		Avatar:   user.Avatar,
		Provider: p.Name(),
		Raw: map[string]any{
			"username":   user.Username,
			"name":       user.Name,
			"avatar_url": user.Avatar,
			"email":      user.Email,
		},
	}, nil
}

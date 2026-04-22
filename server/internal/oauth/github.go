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
	"golang.org/x/oauth2/github"
)

type GitHubProvider struct {
	config *golangoauth.Config
}

type gitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

type gitHubEmail struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
	Primary  bool   `json:"primary"`
}

func NewGitHubProvider(cfg config.Config) *GitHubProvider {
	return &GitHubProvider{config: &golangoauth.Config{
		ClientID:     cfg.GitHubClientID,
		ClientSecret: cfg.GitHubClientSecret,
		RedirectURL:  cfg.GitHubRedirectURL,
		Endpoint:     github.Endpoint,
		Scopes:       []string{"read:user", "user:email"},
	}}
}

func (p *GitHubProvider) Name() string {
	return "github"
}

func (p *GitHubProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *GitHubProvider) Exchange(ctx context.Context, code string) (*golangoauth.Token, error) {
	return p.config.Exchange(ctx, code)
}

func (p *GitHubProvider) FetchProfile(ctx context.Context, token *golangoauth.Token) (*UserProfile, error) {
	client := p.config.Client(ctx, token)
	user, err := fetchGitHubUser(client)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(user.Email) == "" {
		email, err := fetchGitHubPrimaryEmail(client)
		if err != nil {
			return nil, err
		}
		user.Email = email
	}
	if strings.TrimSpace(user.Email) == "" {
		return nil, fmt.Errorf("github account did not provide an email")
	}

	return &UserProfile{
		Email:    strings.ToLower(strings.TrimSpace(user.Email)),
		Subject:  fmt.Sprintf("%d", user.ID),
		Name:     firstNonEmpty(user.Name, user.Login),
		Avatar:   user.AvatarURL,
		Provider: p.Name(),
		Raw: map[string]any{
			"login":      user.Login,
			"name":       user.Name,
			"avatar_url": user.AvatarURL,
			"email":      user.Email,
		},
	}, nil
}

func fetchGitHubUser(client *http.Client) (*gitHubUser, error) {
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch github user: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch github user failed: %s", strings.TrimSpace(string(payload)))
	}
	var user gitHubUser
	if err := json.Unmarshal(payload, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func fetchGitHubPrimaryEmail(client *http.Client) (string, error) {
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch github emails: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode >= 400 {
		return "", fmt.Errorf("fetch github emails failed: %s", strings.TrimSpace(string(payload)))
	}
	var emails []gitHubEmail
	if err := json.Unmarshal(payload, &emails); err != nil {
		return "", err
	}
	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}
	for _, email := range emails {
		if email.Verified {
			return email.Email, nil
		}
	}
	return "", nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

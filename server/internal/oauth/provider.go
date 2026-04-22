package oauth

import (
	"context"

	"golang.org/x/oauth2"
)

type UserProfile struct {
	Email    string         `json:"email"`
	Subject  string         `json:"subject"`
	Name     string         `json:"name"`
	Avatar   string         `json:"avatar"`
	Provider string         `json:"provider"`
	Raw      map[string]any `json:"raw"`
}

type Provider interface {
	Name() string
	AuthCodeURL(string) string
	Exchange(context.Context, string) (*oauth2.Token, error)
	FetchProfile(context.Context, *oauth2.Token) (*UserProfile, error)
}

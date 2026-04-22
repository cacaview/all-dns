package service

import (
	"fmt"
	"time"

	"dns-hub/server/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type TokenClaims struct {
	UserID       uint       `json:"userId"`
	Role         model.Role `json:"role"`
	TokenVersion int        `json:"tokenVersion"`
	Type         TokenType  `json:"type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken      string    `json:"accessToken"`
	AccessExpiresAt  time.Time `json:"accessExpiresAt"`
	RefreshToken     string    `json:"refreshToken"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
}

type TokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenService(secret string, accessTTL, refreshTTL time.Duration) *TokenService {
	return &TokenService{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (s *TokenService) IssuePair(user model.User) (*TokenPair, error) {
	accessExpiresAt := time.Now().UTC().Add(s.accessTTL)
	refreshExpiresAt := time.Now().UTC().Add(s.refreshTTL)

	accessToken, err := s.sign(user, TokenTypeAccess, accessExpiresAt)
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.sign(user, TokenTypeRefresh, refreshExpiresAt)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (s *TokenService) Parse(tokenText string, expected TokenType) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenText, &TokenClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.Type != expected {
		return nil, fmt.Errorf("unexpected token type")
	}
	return claims, nil
}

func (s *TokenService) sign(user model.User, tokenType TokenType, expiresAt time.Time) (string, error) {
	claims := TokenClaims{
		UserID:       user.ID,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		Type:         tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", user.ID),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

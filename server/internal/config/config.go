package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName            string
	AppEnv             string
	Port               string
	FrontendURL        string
	BackendPublicURL   string
	UploadDir          string
	ReminderWebhookURL string
	DevLoginEnabled    bool

	MasterKey           []byte
	JWTSecret           string
	JWTAccessTTL        time.Duration
	JWTRefreshTTL       time.Duration
	PropagationResolvers []string

	DatabaseHost      string
	DatabasePort      string
	DatabaseName      string
	DatabaseUser      string
	DatabasePassword  string
	DatabaseSSLMode   string

	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string
	GitLabClientID     string
	GitLabClientSecret string
	GitLabRedirectURL  string

	StorageType string // "local" (default) or "s3"
	S3Config    S3StorageConfig
}

// S3StorageConfig holds S3-compatible object storage settings.
type S3StorageConfig struct {
	Endpoint        string
	Region          string
	Bucket          string
	KeyPrefix       string
	BaseURL         string
	AccessKeyID     string
	SecretAccessKey string
}

func Load() (Config, error) {
	cfg := Config{
		AppName:            getEnv("APP_NAME", "DNS Hub"),
		AppEnv:             getEnv("APP_ENV", "development"),
		Port:               getEnv("PORT", "8080"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
		BackendPublicURL:   strings.TrimSpace(os.Getenv("BACKEND_PUBLIC_URL")),
		UploadDir:          getEnv("UPLOAD_DIR", "uploads"),
		ReminderWebhookURL: strings.TrimSpace(os.Getenv("REMINDER_WEBHOOK_URL")),
		DevLoginEnabled:    getEnv("DEV_LOGIN_ENABLED", "false") == "true",
		JWTSecret:          os.Getenv("JWT_SECRET"),
		DatabaseHost:       getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:       getEnv("DATABASE_PORT", "5432"),
		DatabaseName:       getEnv("DATABASE_NAME", "dns_hub"),
		DatabaseUser:       getEnv("DATABASE_USER", "postgres"),
		DatabasePassword:   getEnv("DATABASE_PASSWORD", "postgres"),
		DatabaseSSLMode:    getEnv("DATABASE_SSLMODE", "disable"),
		GitHubClientID:     strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID")),
		GitHubClientSecret: strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")),
		GitHubRedirectURL:  strings.TrimSpace(os.Getenv("GITHUB_REDIRECT_URL")),
		GitLabClientID:     strings.TrimSpace(os.Getenv("GITLAB_CLIENT_ID")),
		GitLabClientSecret: strings.TrimSpace(os.Getenv("GITLAB_CLIENT_SECRET")),
		GitLabRedirectURL:  strings.TrimSpace(os.Getenv("GITLAB_REDIRECT_URL")),
		StorageType:        getEnv("STORAGE_TYPE", "local"),
		S3Config: S3StorageConfig{
			Endpoint:        strings.TrimSpace(os.Getenv("S3_ENDPOINT")),
			Region:          getEnv("S3_REGION", "us-east-1"),
			Bucket:          strings.TrimSpace(os.Getenv("S3_BUCKET")),
			KeyPrefix:       getEnv("S3_KEY_PREFIX", "dns-hub/"),
			BaseURL:         strings.TrimSpace(os.Getenv("S3_BASE_URL")),
			AccessKeyID:     strings.TrimSpace(os.Getenv("S3_ACCESS_KEY_ID")),
			SecretAccessKey: strings.TrimSpace(os.Getenv("S3_SECRET_ACCESS_KEY")),
		},
	}

	masterKey, err := parseMasterKey(strings.TrimSpace(os.Getenv("APP_MASTER_KEY")))
	if err != nil {
		return Config{}, err
	}
	cfg.MasterKey = masterKey
	if cfg.BackendPublicURL == "" {
		cfg.BackendPublicURL = "http://localhost:" + cfg.Port
	}

	accessMinutes, err := strconv.Atoi(getEnv("JWT_ACCESS_TTL_MINUTES", "15"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid JWT_ACCESS_TTL_MINUTES: %w", err)
	}
	refreshHours, err := strconv.Atoi(getEnv("JWT_REFRESH_TTL_HOURS", "168"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid JWT_REFRESH_TTL_HOURS: %w", err)
	}
	cfg.JWTAccessTTL = time.Duration(accessMinutes) * time.Minute
	cfg.JWTRefreshTTL = time.Duration(refreshHours) * time.Hour

	resolversEnv := strings.TrimSpace(os.Getenv("PROPAGATION_RESOLVERS"))
	if resolversEnv != "" {
		cfg.PropagationResolvers = strings.Split(resolversEnv, ",")
		for i := range cfg.PropagationResolvers {
			cfg.PropagationResolvers[i] = strings.TrimSpace(cfg.PropagationResolvers[i])
		}
	} else {
		cfg.PropagationResolvers = defaultPropagationResolvers()
	}

	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}

	return cfg, nil
}

func (c Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.DatabaseHost,
		c.DatabasePort,
		c.DatabaseUser,
		c.DatabasePassword,
		c.DatabaseName,
		c.DatabaseSSLMode,
	)
}

func (c Config) OAuthEnabled(provider string) bool {
	switch provider {
	case "github":
		return c.GitHubClientID != "" && c.GitHubClientSecret != "" && c.GitHubRedirectURL != ""
	case "gitlab":
		return c.GitLabClientID != "" && c.GitLabClientSecret != "" && c.GitLabRedirectURL != ""
	default:
		return false
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func parseMasterKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, errors.New("APP_MASTER_KEY is required")
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("APP_MASTER_KEY must be base64 encoded: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("APP_MASTER_KEY must decode to 32 bytes, got %d", len(decoded))
	}

	return decoded, nil
}

func defaultPropagationResolvers() []string {
	return []string{"1.1.1.1:53", "8.8.8.8:53", "114.114.114.114:53", "223.5.5.5:53", "208.67.222.222:53"}
}

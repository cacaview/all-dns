package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	appconfig "dns-hub/server/internal/config"
	appdb "dns-hub/server/internal/db"
	apphttp "dns-hub/server/internal/http"
	"dns-hub/server/internal/notifier"
	appoauth "dns-hub/server/internal/oauth"
	"dns-hub/server/internal/service"
	"dns-hub/server/internal/storage"
)

func main() {
	cfg, err := appconfig.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err := appdb.Connect(cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	if err := appdb.Migrate(database); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("create upload dir: %v", err)
	}

	cryptoService := service.NewCryptoService(cfg.MasterKey)
	tokenService := service.NewTokenService(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	webhookNotifier := notifier.NewWebhookNotifier(cfg.ReminderWebhookURL)
	emailNotifier := notifier.NewEmailNotifier(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	webhookService := service.NewWebhookService(database, webhookNotifier)
	snapshotService := service.NewSnapshotService(database)
	propagationService := service.NewPropagationService(database, cfg.PropagationResolvers)
	reminderService := service.NewReminderService(database, webhookService, emailNotifier)
	dnsService := service.NewDNSService(database, cryptoService, snapshotService, propagationService, reminderService)
	reminderService.SetDNSService(dnsService)

	oauthProviders := []appoauth.Provider{}
	if cfg.OAuthEnabled("github") {
		oauthProviders = append(oauthProviders, appoauth.NewGitHubProvider(cfg))
	}
	if cfg.OAuthEnabled("gitlab") {
		oauthProviders = append(oauthProviders, appoauth.NewGitLabProvider(cfg))
	}
	authService := service.NewAuthService(database, tokenService, oauthProviders...)

	// Build storage based on configured type
	var fileStorage storage.Storage
	if cfg.StorageType == "s3" && cfg.S3Config.Bucket != "" {
		ctx := context.Background()
		fileStorage, err = storage.NewS3Storage(ctx, storage.S3Config{
			Endpoint:        cfg.S3Config.Endpoint,
			Region:          cfg.S3Config.Region,
			Bucket:          cfg.S3Config.Bucket,
			KeyPrefix:       cfg.S3Config.KeyPrefix,
			BaseURL:         cfg.S3Config.BaseURL,
			AccessKeyID:     cfg.S3Config.AccessKeyID,
			SecretAccessKey: cfg.S3Config.SecretAccessKey,
		})
		if err != nil {
			log.Fatalf("init s3 storage: %v", err)
		}
		log.Printf("using S3 storage bucket: %s", cfg.S3Config.Bucket)
	} else {
		fileStorage = storage.NewLocalStorage(cfg.UploadDir, cfg.BackendPublicURL+"/uploads")
		log.Printf("using local storage at: %s", cfg.UploadDir)
	}

	router := apphttp.NewRouter(cfg, database, authService, tokenService, dnsService, webhookService, fileStorage)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	reminderService.Start(ctx)

	go func() {
		log.Printf("starting %s on :%s", cfg.AppName, cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

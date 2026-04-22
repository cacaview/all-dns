package http

import (
	"dns-hub/server/internal/config"
	"dns-hub/server/internal/http/handler"
	"dns-hub/server/internal/http/middleware"
	"dns-hub/server/internal/model"
	"dns-hub/server/internal/service"
	"dns-hub/server/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter(cfg config.Config, authService *service.AuthService, tokenService *service.TokenService, dnsService *service.DNSService, fileStorage storage.Storage) *gin.Engine {
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.FrontendURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	router.MaxMultipartMemory = 16 << 20
	// Only serve local uploads dir when using local storage
	if cfg.StorageType != "s3" {
		router.StaticFS("/uploads", gin.Dir(cfg.UploadDir, false))
	}

	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(cfg, authService)
	userHandler := handler.NewUserHandler(authService)
	accountHandler := handler.NewAccountHandler(dnsService)
	domainHandler := handler.NewDomainHandler(dnsService)
	snapshotHandler := handler.NewSnapshotHandler(dnsService)
	profileHandler := handler.NewProfileHandler(dnsService, fileStorage)

	authMiddleware := middleware.NewAuthMiddleware(tokenService, authService)
	rbac := middleware.NewRBACMiddleware()

	router.GET("/health", healthHandler.Get)

	api := router.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.GET("/oauth/:provider/login", authHandler.StartOAuth)
			auth.GET("/oauth/:provider/callback", authHandler.CompleteOAuth)
			auth.POST("/refresh", authHandler.Refresh)
			auth.POST("/dev-login", authHandler.DevLogin)
		}

		protected := api.Group("")
		protected.Use(authMiddleware.RequireAuth())
		{
			protected.POST("/auth/logout", authHandler.Logout)
			protected.GET("/auth/me", authHandler.Me)

			// User management — admin only
			admins := protected.Group("")
			admins.Use(rbac.RequireRoles(model.RoleAdmin))
			{
				admins.GET("/users", userHandler.List)
				admins.PUT("/users/:id/role", userHandler.UpdateRole)
			}

			protected.GET("/dashboard/summary", domainHandler.Summary)
			protected.GET("/accounts", accountHandler.List)
			protected.GET("/accounts/providers", accountHandler.Providers)
			protected.GET("/accounts/reminders", accountHandler.Reminders)
			protected.PUT("/accounts/:id/reminder-handled", accountHandler.SetReminderHandled)
			protected.POST("/accounts/:id/validate", accountHandler.Validate)
			protected.GET("/domains", domainHandler.List)
			protected.GET("/domains/backups", domainHandler.ListAllBackups)
			protected.GET("/domains/:id/records", domainHandler.ListRecords)
			protected.GET("/domains/:id/backups", snapshotHandler.ListByDomain)
			protected.GET("/domains/:id/profile", profileHandler.Get)
			protected.GET("/domains/propagation-history", domainHandler.ListPropagationHistory)
			protected.GET("/backups/:backupId/export", domainHandler.ExportBackup)

			editors := protected.Group("")
			editors.Use(rbac.RequireRoles(model.RoleAdmin, model.RoleEditor))
			{
				editors.POST("/accounts", accountHandler.Create)
				editors.PUT("/accounts/:id", accountHandler.Update)
				editors.POST("/accounts/:id/rotate", accountHandler.Rotate)
				editors.POST("/domains/:id/star", domainHandler.ToggleStar)
				editors.PUT("/domains/:id/tags", domainHandler.UpdateTags)
				editors.PUT("/domains/:id/archive", domainHandler.SetArchived)
				editors.POST("/domains/:id/records/upsert", domainHandler.UpsertRecord)
				editors.POST("/domains/:id/records/delete", domainHandler.DeleteRecord)
				editors.POST("/domains/:id/propagation-check", domainHandler.TriggerPropagation)
				editors.POST("/backups/:backupId/restore", domainHandler.RestoreBackup)
				editors.POST("/domains/:id/profile/attachments", profileHandler.UploadAttachment)
				editors.PUT("/domains/:id/profile", profileHandler.Update)
			}
		}
	}

	return router
}

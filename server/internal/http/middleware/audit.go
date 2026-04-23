package middleware

import (
	"strings"
	"time"

	"dns-hub/server/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuditLogger writes a structured audit log entry for each authenticated request.
type AuditLogger struct {
	db *gorm.DB
}

// NewAuditLogger creates an AuditLogger.
func NewAuditLogger(db *gorm.DB) *AuditLogger {
	return &AuditLogger{db: db}
}

// Log returns a Gin middleware that records audit events for authenticated requests.
// Only non-GET requests are logged by default to avoid flooding with read-only traffic.
func (a *AuditLogger) Log(nonGETOnly bool) func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Next()

		// Only log authenticated, non-GET requests by default
		method := c.Request.Method
		if nonGETOnly && (method == "GET" || method == "HEAD" || method == "OPTIONS") {
			return
		}
		user, ok := CurrentUser(c)
		if !ok {
			return
		}

		statusCode := c.Writer.Status()
		// Ignore health checks and auth endpoints
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/health") || strings.HasPrefix(path, "/api/v1/auth/") {
			return
		}

		log := model.AuditLog{
			UserID:        user.ID,
			OrgID:         user.PrimaryOrgID,
			Action:        actionFromMethod(method),
			Resource:      resourceFromPath(path),
			ResourceID:    resourceIDFromPath(c),
			Description:   descriptionFromRequest(method, path, statusCode),
			IPAddress:     c.ClientIP(),
			UserAgent:     c.GetHeader("User-Agent"),
			RequestPath:   path,
			RequestMethod: method,
			StatusCode:    statusCode,
			CreatedAt:     time.Now().UTC(),
		}
		// Log asynchronously to avoid blocking the response
		go func() {
			_ = a.db.Create(&log).Error
		}()
	}
}

func actionFromMethod(method string) string {
	switch method {
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	case "GET":
		return "read"
	default:
		return method
	}
}

func resourceFromPath(path string) string {
	// path format: /api/v1/{resource}[/{id}[/...]]
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "unknown"
	}
	switch parts[0] {
	case "dashboard":
		return "dashboard"
	case "accounts":
		return "account"
	case "domains":
		return "domain"
	case "backups":
		return "backup"
	case "webhooks":
		return "webhook"
	case "users":
		return "user"
	default:
		return parts[0]
	}
}

func resourceIDFromPath(c *gin.Context) uint {
	// Try to extract numeric ID from common positions
	// For paths like /api/v1/domains/123/records
	path := c.Request.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	for _, part := range parts {
		var id uint
		if _, err := parseUintParamSafe(part); err == nil {
			id, _ = parseUintParamSafe(part)
			return id
		}
	}
	return 0
}

func parseUintParamSafe(s string) (uint, error) {
	var v uint
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		v = v*10 + uint(c-'0')
	}
	return v, nil
}

func descriptionFromRequest(method, path string, statusCode int) string {
	if statusCode >= 400 {
		return "request failed"
	}
	return "request completed"
}

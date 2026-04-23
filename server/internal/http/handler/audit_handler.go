package handler

import (
	"net/http"
	"strconv"

	"dns-hub/server/internal/http/middleware"
	"dns-hub/server/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuditHandler struct {
	db *gorm.DB
}

func NewAuditHandler(db *gorm.DB) *AuditHandler {
	return &AuditHandler{db: db}
}

func (h *AuditHandler) List(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	// Admin-only
	if user.Role != model.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	offset := 0
	if raw := c.Query("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	var logs []model.AuditLog
	query := h.db.Model(&model.AuditLog{}).Order("created_at desc").Limit(limit).Offset(offset)
	if user.Role != model.RoleAdmin {
		query = query.Where("org_id = ?", user.PrimaryOrgID)
	}
	if resource := c.Query("resource"); resource != "" {
		query = query.Where("resource = ?", resource)
	}
	if resourceID, err := strconv.ParseUint(c.Query("resourceId"), 10, 64); err == nil {
		query = query.Where("resource_id = ?", resourceID)
	}
	if userID, err := strconv.ParseUint(c.Query("userId"), 10, 64); err == nil {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var total int64
	countQuery := h.db.Model(&model.AuditLog{})
	if user.Role != model.RoleAdmin {
		countQuery = countQuery.Where("org_id = ?", user.PrimaryOrgID)
	}
	if resource := c.Query("resource"); resource != "" {
		countQuery = countQuery.Where("resource = ?", resource)
	}
	if resourceID, err := strconv.ParseUint(c.Query("resourceId"), 10, 64); err == nil {
		countQuery = countQuery.Where("resource_id = ?", resourceID)
	}
	if userID, err := strconv.ParseUint(c.Query("userId"), 10, 64); err == nil {
		countQuery = countQuery.Where("user_id = ?", userID)
	}
	countQuery.Count(&total)
	c.JSON(http.StatusOK, gin.H{
		"items":  logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"dns-hub/server/internal/http/middleware"
	"dns-hub/server/internal/provider"
	"dns-hub/server/internal/service"
	"github.com/gin-gonic/gin"
)

type DomainHandler struct {
	dns *service.DNSService
}

func NewDomainHandler(dns *service.DNSService) *DomainHandler {
	return &DomainHandler{dns: dns}
}

func (h *DomainHandler) Summary(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	summary, err := h.dns.GetDashboardSummary(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": summary})
}

func (h *DomainHandler) List(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	includeArchived := strings.EqualFold(strings.TrimSpace(c.Query("includeArchived")), "true")
	items, err := h.dns.ListDomainsWithOptions(user.ID, service.DomainListOptions{
		Search:          c.Query("search"),
		IncludeArchived: includeArchived,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *DomainHandler) SetArchived(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	domainID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var request struct {
		Archived bool `json:"archived"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.dns.SetDomainArchived(user.ID, domainID, request.Archived)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

func (h *DomainHandler) RestoreBackup(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	backupID, err := parseUintParam(c, "backupId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	backup, err := h.dns.RestoreBackup(c.Request.Context(), user.ID, backupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": backup})
}

func (h *DomainHandler) ExportBackup(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	backupID, err := parseUintParam(c, "backupId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload, filename, err := h.dns.ExportBackup(user.ID, backupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, "application/json", payload)
}

func (h *DomainHandler) ListAllBackups(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	items, err := h.dns.ListAllBackups(user.ID, c.Query("search"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *DomainHandler) ListPropagationHistory(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	var domainID uint
	if rawID := strings.TrimSpace(c.Query("domainId")); rawID != "" {
		parsed, err := strconv.ParseUint(rawID, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domainId"})
			return
		}
		domainID = uint(parsed)
	}
	items, err := h.dns.ListPropagationHistory(user.ID, domainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *DomainHandler) ToggleStar(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	domainID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.dns.ToggleDomainStar(user.ID, domainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

func (h *DomainHandler) UpdateTags(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	domainID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var request struct {
		Tags []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.dns.UpdateDomainTags(user.ID, domainID, request.Tags)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

func (h *DomainHandler) ListRecords(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	domainID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items, err := h.dns.ListRecords(c.Request.Context(), user.ID, domainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *DomainHandler) UpsertRecord(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	domainID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var request provider.RecordMutation
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, backup, propagation, err := h.dns.UpsertRecord(c.Request.Context(), user.ID, domainID, request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "backup": backup})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item, "backup": backup, "propagation": propagation})
}

func (h *DomainHandler) DeleteRecord(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	domainID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var request struct {
		RecordID string `json:"recordId"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	backup, err := h.dns.DeleteRecord(c.Request.Context(), user.ID, domainID, strings.TrimSpace(request.RecordID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "backup": backup})
		return
	}
	c.JSON(http.StatusOK, gin.H{"backup": backup})
}

func (h *DomainHandler) TriggerPropagation(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	domainID, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var request struct {
		provider.RecordMutation
		Resolvers      []string `json:"resolvers"`       // optional: override default resolvers
		Watch          bool     `json:"watch"`            // if true, poll continuously
		WatchInterval  int      `json:"watchInterval"`    // polling interval seconds, default 30
		WatchMaxAttempts int    `json:"watchMaxAttempts"` // max attempts, default 20
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.dns.TriggerPropagationCheckWithOptions(c.Request.Context(), user.ID, domainID, request.RecordMutation, service.WatchOptions{
		Resolvers:     request.Resolvers,
		IntervalSecs:  request.WatchInterval,
		MaxAttempts:   request.WatchMaxAttempts,
	}, request.Watch)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": result})
}

func parseUintParam(c *gin.Context, name string) (uint, error) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(value), nil
}

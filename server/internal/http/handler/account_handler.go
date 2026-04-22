package handler

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"dns-hub/server/internal/http/middleware"
	"dns-hub/server/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountHandler struct {
	dns *service.DNSService
}

func NewAccountHandler(dns *service.DNSService) *AccountHandler {
	return &AccountHandler{dns: dns}
}

func (h *AccountHandler) List(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	accounts, err := h.dns.ListAccounts(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": accounts})
}

func (h *AccountHandler) Create(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	input, ok := parseAccountInput(c)
	if !ok {
		return
	}
	account, err := h.dns.CreateAccount(c.Request.Context(), user.ID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"item": account})
}

func (h *AccountHandler) Update(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	accountID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	input, ok := parseAccountInput(c)
	if !ok {
		return
	}
	account, err := h.dns.UpdateAccount(c.Request.Context(), user.ID, uint(accountID), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": account})
}

func (h *AccountHandler) Validate(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	accountID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	result, err := h.dns.ValidateAndSyncAccount(c.Request.Context(), user.ID, uint(accountID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": result})
}

func (h *AccountHandler) Reminders(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	items, err := h.dns.ListReminders(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AccountHandler) SetReminderHandled(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	accountID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	var request struct {
		Handled bool `json:"handled"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.dns.SetReminderHandled(user.ID, uint(accountID), request.Handled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AccountHandler) Rotate(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	accountID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	input, ok := parseAccountInput(c)
	if !ok {
		return
	}
	account, result, err := h.dns.RotateAccountCredentials(c.Request.Context(), user.ID, uint(accountID), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "item": account})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": account, "validation": result})
}

func (h *AccountHandler) Providers(c *gin.Context) {
	items := h.dns.ListProviderCatalog()
	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func parseAccountInput(c *gin.Context) (service.AccountInput, bool) {
	var request struct {
		Name      string         `json:"name"`
		Provider  string         `json:"provider"`
		Config    map[string]any `json:"config"`
		ExpiresAt string         `json:"expiresAt"`
		Status    string         `json:"status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return service.AccountInput{}, false
	}
	var expiresAt *time.Time
	if strings.TrimSpace(request.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, request.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expiresAt must be RFC3339"})
			return service.AccountInput{}, false
		}
		expiresAt = &parsed
	}
	return service.AccountInput{
		Name:      request.Name,
		Provider:  request.Provider,
		Config:    request.Config,
		ExpiresAt: expiresAt,
		Status:    request.Status,
	}, true
}

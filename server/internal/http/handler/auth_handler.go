package handler

import (
	"net/http"
	"strings"

	"dns-hub/server/internal/config"
	"dns-hub/server/internal/http/middleware"
	"dns-hub/server/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	cfg  config.Config
	auth *service.AuthService
}

func NewAuthHandler(cfg config.Config, auth *service.AuthService) *AuthHandler {
	return &AuthHandler{cfg: cfg, auth: auth}
}

func (h *AuthHandler) StartOAuth(c *gin.Context) {
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	url, err := h.auth.StartAuth(provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *AuthHandler) CompleteOAuth(c *gin.Context) {
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	state := strings.TrimSpace(c.Query("state"))
	code := strings.TrimSpace(c.Query("code"))
	if state == "" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state and code are required"})
		return
	}
	result, err := h.auth.CompleteAuth(c.Request.Context(), provider, state, code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, h.auth.RedirectURL(h.cfg.FrontendURL, result))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var request struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.auth.Refresh(request.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AuthHandler) DevLogin(c *gin.Context) {
	if !h.cfg.DevLoginEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "dev login is disabled"})
		return
	}
	var request struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.auth.DevLogin(request.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	if err := h.auth.Logout(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

package handler

import (
	"net/http"

	"dns-hub/server/internal/http/middleware"
	"dns-hub/server/internal/service"
	"github.com/gin-gonic/gin"
)

type SnapshotHandler struct {
	dns *service.DNSService
}

func NewSnapshotHandler(dns *service.DNSService) *SnapshotHandler {
	return &SnapshotHandler{dns: dns}
}

func (h *SnapshotHandler) ListByDomain(c *gin.Context) {
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
	items, err := h.dns.ListBackups(user.ID, domainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

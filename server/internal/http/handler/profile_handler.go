package handler

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"dns-hub/server/internal/http/middleware"
	"dns-hub/server/internal/service"
	"dns-hub/server/internal/storage"
	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	dns     *service.DNSService
	storage storage.Storage
}

func NewProfileHandler(dns *service.DNSService, storage storage.Storage) *ProfileHandler {
	return &ProfileHandler{dns: dns, storage: storage}
}

func (h *ProfileHandler) Get(c *gin.Context) {
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
	item, err := h.dns.GetDomainProfile(user.ID, domainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

func (h *ProfileHandler) Update(c *gin.Context) {
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
		Description    string   `json:"description"`
		AttachmentURLs []string `json:"attachmentUrls"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.dns.UpdateDomainProfile(user.ID, domainID, request.Description, request.AttachmentURLs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

func (h *ProfileHandler) UploadAttachment(c *gin.Context) {
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
	if _, err := h.dns.GetDomainProfile(user.ID, domainID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	fileName := sanitizeFileName(file.Filename)
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file name"})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
		return
	}
	data, err := io.ReadAll(src)
	src.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
		return
	}
	key := fmt.Sprintf("domain-%d/%d-%s", domainID, time.Now().UnixNano(), fileName)
	url, err := h.storage.Upload(c.Request.Context(), key, data, file.Header.Get("Content-Type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": gin.H{"name": fileName, "url": url}})
}

func sanitizeFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, " ", "-")
	builder := strings.Builder{}
	for _, char := range base {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '.' || char == '-' || char == '_' {
			builder.WriteRune(char)
		}
	}
	return strings.Trim(builder.String(), ".-")
}

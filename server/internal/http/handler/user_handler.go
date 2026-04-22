package handler

import (
	"net/http"
	"strconv"

	"dns-hub/server/internal/model"
	"dns-hub/server/internal/service"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	auth *service.AuthService
}

func NewUserHandler(auth *service.AuthService) *UserHandler {
	return &UserHandler{auth: auth}
}

func (h *UserHandler) List(c *gin.Context) {
	var users []model.User
	if err := h.auth.ListUsers(&users); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": users})
}

func (h *UserHandler) UpdateRole(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var request struct {
		Role model.Role `json:"role"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if request.Role != model.RoleAdmin && request.Role != model.RoleEditor && request.Role != model.RoleViewer {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be admin, editor, or viewer"})
		return
	}
	if err := h.auth.UpdateUserRole(uint(userID), request.Role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

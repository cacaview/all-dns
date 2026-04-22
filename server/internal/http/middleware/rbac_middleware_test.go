package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dns-hub/server/internal/model"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestRBAC_AdminCanAccess(t *testing.T) {
	rbac := NewRBACMiddleware()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("currentUser", &model.User{ID: 1, Role: model.RoleAdmin})
		c.Next()
	})
	router.GET("/admin", rbac.RequireRoles(model.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("admin should get 200, got %d", w.Code)
	}
}

func TestRBAC_ViewerCannotAccessAdminRoute(t *testing.T) {
	rbac := NewRBACMiddleware()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("currentUser", &model.User{ID: 2, Role: model.RoleViewer})
		c.Next()
	})
	router.GET("/admin", rbac.RequireRoles(model.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("viewer should get 403, got %d", w.Code)
	}
}

func TestRBAC_EditorCanAccessEditorRoute(t *testing.T) {
	rbac := NewRBACMiddleware()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("currentUser", &model.User{ID: 3, Role: model.RoleEditor})
		c.Next()
	})
	router.GET("/editor", rbac.RequireRoles(model.RoleAdmin, model.RoleEditor), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/editor", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("editor should get 200, got %d", w.Code)
	}
}

func TestRBAC_NoUser(t *testing.T) {
	rbac := NewRBACMiddleware()
	router := gin.New()
	router.GET("/admin", rbac.RequireRoles(model.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing user should get 401, got %d", w.Code)
	}
}

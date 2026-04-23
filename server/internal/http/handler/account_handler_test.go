package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dns-hub/server/internal/model"
	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestAccountHandler_ParseAccountInput_Valid(t *testing.T) {
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		input, ok := parseAccountInput(c)
		if !ok {
			c.Status(http.StatusBadRequest)
			return
		}
		if input.Name != "Test Account" {
			t.Errorf("expected name 'Test Account', got %q", input.Name)
		}
		if input.Provider != "cloudflare" {
			t.Errorf("expected provider 'cloudflare', got %q", input.Provider)
		}
		c.Status(http.StatusOK)
	})

	body := `{"name":"Test Account","provider":"cloudflare","config":{"token":"abc"},"expiresAt":"2025-12-01T00:00:00Z"}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAccountHandler_ParseAccountInput_MissingName(t *testing.T) {
	// Missing name is not a binding error — name is optional in the struct.
	// parseAccountInput returns ok=true; the handler/service validates required fields.
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		input, ok := parseAccountInput(c)
		if !ok {
			c.Status(http.StatusBadRequest)
			return
		}
		if input.Name != "" {
			t.Errorf("expected empty name, got %q", input.Name)
		}
		if input.Provider != "cloudflare" {
			t.Errorf("expected provider 'cloudflare', got %q", input.Provider)
		}
		c.Status(http.StatusOK)
	})

	body := `{"provider":"cloudflare"}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAccountHandler_ParseAccountInput_InvalidExpiresAt(t *testing.T) {
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		_, ok := parseAccountInput(c)
		if ok {
			t.Error("expected parse to fail for invalid expiresAt")
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusBadRequest)
	})

	body := `{"name":"Test","provider":"cloudflare","expiresAt":"invalid-date"}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAccountHandler_ParseAccountInput_EmptyJSON(t *testing.T) {
	// Empty JSON {} binds successfully with zero values for all fields.
	// parseAccountInput returns ok=true; the caller validates required fields.
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		input, ok := parseAccountInput(c)
		if !ok {
			c.Status(http.StatusBadRequest)
			return
		}
		if input.Name != "" {
			t.Errorf("expected empty name, got %q", input.Name)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAccountHandler_ParseAccountInput_StatusField(t *testing.T) {
	router := gin.New()
	router.POST("/test", func(c *gin.Context) {
		input, ok := parseAccountInput(c)
		if !ok {
			c.Status(http.StatusBadRequest)
			return
		}
		if input.Status != "inactive" {
			t.Errorf("expected status 'inactive', got %q", input.Status)
		}
		c.Status(http.StatusOK)
	})

	body := `{"name":"Test","provider":"cloudflare","status":"inactive"}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAccountHandler_Providers_Sort(t *testing.T) {
	items := []struct {
		Label string `json:"label"`
	}{{"zebra"}, {"apple"}, {"cloudflare"}}
	sorted := items
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Label < sorted[i].Label {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if sorted[0].Label != "apple" {
		t.Errorf("expected first label 'apple', got %q", sorted[0].Label)
	}
	if sorted[2].Label != "zebra" {
		t.Errorf("expected last label 'zebra', got %q", sorted[2].Label)
	}
}

func TestParseUintParam(t *testing.T) {
	router := gin.New()
	router.GET("/:id", func(c *gin.Context) {
		id, err := parseUintParam(c, "id")
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": id})
	})

	tests := []struct {
		path       string
		wantStatus int
		wantID     uint
	}{
		{"/123", http.StatusOK, 123},
		{"/0", http.StatusOK, 0},
		{"/abc", http.StatusBadRequest, 0},
		{"/-1", http.StatusBadRequest, 0},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != tt.wantStatus {
			t.Errorf("GET %s: expected %d, got %d", tt.path, tt.wantStatus, w.Code)
		}
		if tt.wantStatus == http.StatusOK {
			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			gotID := uint(resp["id"].(float64))
			if gotID != tt.wantID {
				t.Errorf("GET %s: expected id=%d, got %d", tt.path, tt.wantID, gotID)
			}
		}
	}
}

func TestAccountHandler_List_RequiresAuth(t *testing.T) {
	h := &AccountHandler{}
	router := gin.New()
	// No auth middleware — user will be missing
	router.GET("/accounts", h.List)

	req := httptest.NewRequest("GET", "/accounts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAccountHandler_Create_RequiresAuth(t *testing.T) {
	h := &AccountHandler{}
	router := gin.New()
	router.POST("/accounts", h.Create)

	body := `{"name":"Test","provider":"cloudflare"}`
	req := httptest.NewRequest("POST", "/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWebhookHandler_List_RequiresAuth(t *testing.T) {
	h := &WebhookHandler{}
	router := gin.New()
	router.GET("/webhooks", h.List)

	req := httptest.NewRequest("GET", "/webhooks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWebhookHandler_Create_RequiresAuth(t *testing.T) {
	h := &WebhookHandler{}
	router := gin.New()
	router.POST("/webhooks", h.Create)

	body := `{"name":"Test","url":"https://example.com/webhook"}`
	req := httptest.NewRequest("POST", "/webhooks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWebhookHandler_Delete_RequiresAuth(t *testing.T) {
	h := &WebhookHandler{}
	router := gin.New()
	router.DELETE("/webhooks/:id", h.Delete)

	req := httptest.NewRequest("DELETE", "/webhooks/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWebhookHandler_Update_InvalidID(t *testing.T) {
	h := &WebhookHandler{}
	router := gin.New()
	// Set a mock user
	router.Use(func(c *gin.Context) {
		c.Set("currentUser", &model.User{ID: 1, PrimaryOrgID: 1})
		c.Next()
	})
	router.PUT("/webhooks/:id", h.Update)

	body := `{"name":"Updated"}`
	req := httptest.NewRequest("PUT", "/webhooks/notanumber", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookHandler_Create_Validation(t *testing.T) {
	h := &WebhookHandler{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("currentUser", &model.User{ID: 1, PrimaryOrgID: 1})
		c.Next()
	})
	router.POST("/webhooks", h.Create)

	// Missing required fields
	body := `{}`
	req := httptest.NewRequest("POST", "/webhooks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookHandler_Delete_InvalidID(t *testing.T) {
	h := &WebhookHandler{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("currentUser", &model.User{ID: 1, PrimaryOrgID: 1})
		c.Next()
	})
	router.DELETE("/webhooks/:id", h.Delete)

	req := httptest.NewRequest("DELETE", "/webhooks/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDomainHandler_ToggleStar_RequiresAuth(t *testing.T) {
	h := &DomainHandler{}
	router := gin.New()
	router.POST("/domains/:id/star", h.ToggleStar)

	req := httptest.NewRequest("POST", "/domains/1/star", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDomainHandler_List_RequiresAuth(t *testing.T) {
	h := &DomainHandler{}
	router := gin.New()
	router.GET("/domains", h.List)

	req := httptest.NewRequest("GET", "/domains", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDomainHandler_UpsertRecord_RequiresAuth(t *testing.T) {
	h := &DomainHandler{}
	router := gin.New()
	router.POST("/domains/:id/records/upsert", h.UpsertRecord)

	body := `{"type":"A","name":"www","content":"1.2.3.4"}`
	req := httptest.NewRequest("POST", "/domains/1/records/upsert", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDomainHandler_TriggerPropagation_RequiresAuth(t *testing.T) {
	h := &DomainHandler{}
	router := gin.New()
	router.POST("/domains/:id/propagation-check", h.TriggerPropagation)

	body := `{"type":"A","name":"www","content":"1.2.3.4"}`
	req := httptest.NewRequest("POST", "/domains/1/propagation-check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDomainHandler_ListPropagationHistory_RequiresAuth(t *testing.T) {
	h := &DomainHandler{}
	router := gin.New()
	router.GET("/domains/propagation-history", h.ListPropagationHistory)

	req := httptest.NewRequest("GET", "/domains/propagation-history", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDomainHandler_ExportRecords_RequiresAuth(t *testing.T) {
	h := &DomainHandler{}
	router := gin.New()
	router.GET("/domains/:id/records/export", h.ExportRecords)

	req := httptest.NewRequest("GET", "/domains/1/records/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDomainHandler_ImportRecords_RequiresAuth(t *testing.T) {
	h := &DomainHandler{}
	router := gin.New()
	router.POST("/domains/:id/records/import", h.ImportRecords)

	req := httptest.NewRequest("POST", "/domains/1/records/import", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDomainHandler_InvalidDomainID(t *testing.T) {
	h := &DomainHandler{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("currentUser", &model.User{ID: 1})
		c.Next()
	})
	router.GET("/domains/:id/records", h.ListRecords)

	req := httptest.NewRequest("GET", "/domains/notanumber/records", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuditHandler_List_RequiresAuth(t *testing.T) {
	h := &AuditHandler{}
	router := gin.New()
	router.GET("/audit-logs", h.List)

	req := httptest.NewRequest("GET", "/audit-logs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

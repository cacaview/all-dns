package middleware

import (
	"net/http"
	"strings"

	"dns-hub/server/internal/model"
	"dns-hub/server/internal/service"
	"github.com/gin-gonic/gin"
)

const currentUserKey = "currentUser"

type AuthMiddleware struct {
	tokens *service.TokenService
	auth   *service.AuthService
}

func NewAuthMiddleware(tokens *service.TokenService, auth *service.AuthService) *AuthMiddleware {
	return &AuthMiddleware{tokens: tokens, auth: auth}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		tokenText := strings.TrimSpace(authorization[7:])
		claims, err := m.tokens.Parse(tokenText, service.TokenTypeAccess)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		user, err := m.auth.GetUserByID(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
		if user.TokenVersion != claims.TokenVersion {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
			return
		}
		c.Set(currentUserKey, user)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (*model.User, bool) {
	value, ok := c.Get(currentUserKey)
	if !ok {
		return nil, false
	}
	user, ok := value.(*model.User)
	return user, ok
}

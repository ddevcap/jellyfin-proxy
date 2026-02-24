package handler

import (
	"github.com/ddevcap/jellyfin-proxy/api/middleware"
	"github.com/ddevcap/jellyfin-proxy/ent"
	"github.com/gin-gonic/gin"
)

// userFromCtx extracts the authenticated proxy user from the gin context.
func userFromCtx(c *gin.Context) *ent.User {
	u, _ := c.Get(middleware.ContextKeyUser)
	user, _ := u.(*ent.User)
	return user
}

func fallback(s, def string) string {
	if s != "" {
		return s
	}
	return def
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

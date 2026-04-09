package middleware

import (
	adminauthapp "edge-pilot/internal/adminauth/application"
	"edge-pilot/internal/shared/config"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/common/result"
)

const (
	adminSessionKey  = "ep.admin.session"
	adminUsernameKey = "ep.admin.username"
)

func ApplyProxyTrust(engine *gin.Engine, cfg *config.AdminAuthConfig) error {
	if err := engine.SetTrustedProxies(cfg.TrustedProxyCIDRs); err != nil {
		return err
	}
	if cfg.TrustCloudflare {
		engine.TrustedPlatform = gin.PlatformCloudflare
	}
	return nil
}

func RequireAdminSession(auth *adminauthapp.Service, cfg *config.AdminAuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cfg.CookieName)
		if err != nil || strings.TrimSpace(token) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, result.Custom[any](http.StatusUnauthorized, "Unauthorized", nil))
			return
		}
		claims, err := auth.ParseSession(token)
		if err != nil {
			clearSessionCookie(c, cfg)
			c.AbortWithStatusJSON(http.StatusUnauthorized, result.Custom[any](http.StatusUnauthorized, "Unauthorized", nil))
			return
		}
		c.Set(adminSessionKey, claims)
		c.Set(adminUsernameKey, claims.Username)
		c.Next()
	}
}

func CurrentAdminSession(c *gin.Context) (*adminauthapp.SessionClaims, bool) {
	value, ok := c.Get(adminSessionKey)
	if !ok {
		return nil, false
	}
	claims, ok := value.(*adminauthapp.SessionClaims)
	return claims, ok
}

func CurrentAdminUsername(c *gin.Context) string {
	value, ok := c.Get(adminUsernameKey)
	if !ok {
		return ""
	}
	username, ok := value.(string)
	if !ok {
		return ""
	}
	return username
}

func SetSessionCookie(c *gin.Context, cfg *config.AdminAuthConfig, value string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   isForwardedHTTPS(c, cfg),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(c *gin.Context, cfg *config.AdminAuthConfig) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cfg.CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isForwardedHTTPS(c, cfg),
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(c *gin.Context, cfg *config.AdminAuthConfig) {
	clearSessionCookie(c, cfg)
}

func isForwardedHTTPS(c *gin.Context, cfg *config.AdminAuthConfig) bool {
	if c.Request.TLS != nil {
		return true
	}
	if !cfg.IsTrustedProxy(c.Request.RemoteAddr) {
		return false
	}
	if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		return true
	}
	for _, part := range strings.Split(c.GetHeader("Forwarded"), ";") {
		if strings.EqualFold(strings.TrimSpace(part), "proto=https") {
			return true
		}
	}
	return false
}

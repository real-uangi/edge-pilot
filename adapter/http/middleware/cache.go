/*
 * Copyright 2025 Uangi. All rights reserved.
 * @author uangi
 * @date 2025/7/24 17:06
 */

// Package middleware

package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

var needCacheRes = map[string]int{
	"/favicon.ico":   3600,
	"/manifest.json": 86400,
	"/robots.txt":    86400,
}

func AssignCache(c *gin.Context) {
	path := c.Request.RequestURI
	if strings.HasPrefix(path, "/api/") {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0, private")
		c.Header("Surrogate-Control", "no-store, max-age=0")
	} else if strings.HasPrefix(path, "/assets/") {
		c.Header("Cache-Control", "public, max-age=604800")
	} else if ttl, ok := needCacheRes[path]; ok {
		c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", ttl))
	}
	c.Next()
}

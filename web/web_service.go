/*
 * Copyright 2026 Uangi. All rights reserved.
 * @author uangi
 * @date 2026/1/22 10:44
 */

// Package web

package web

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/real-uangi/allingo/common/business"
	"github.com/real-uangi/allingo/common/env"
	"github.com/real-uangi/allingo/common/result"
	"github.com/real-uangi/fxtrategy"
)

type StaticWebService struct {
	httpFS        http.FileSystem
	fileServer    http.Handler
	indexPageData []byte
}

func newStaticWebService(fsCtx *fxtrategy.Context[Design]) (*StaticWebService, error) {
	webTheme := env.GetOrDefault("WEB_THEME", ThemeDefault)
	design, ok := fsCtx.Get(webTheme)
	if !ok {
		return nil, fmt.Errorf("web design for theme '%s' not found", webTheme)
	}
	httpFS := design.GetFS()
	index, err := httpFS.Open("index.html")
	if err != nil {
		return nil, business.NewErrorf("precheck failed. index file does not exist: %s", err.Error())
	}
	indexPageData, err := io.ReadAll(index)
	if err != nil {
		return nil, err
	}
	return &StaticWebService{
		httpFS:        httpFS,
		fileServer:    http.StripPrefix("/", http.FileServer(httpFS)),
		indexPageData: indexPageData,
	}, nil
}

func (ws *StaticWebService) NoRoute(c *gin.Context) {
	if strings.HasPrefix(c.Request.RequestURI, "/api") {
		c.Render(http.StatusNotFound, result.NotFound())
		return
	}
	c.Header("Cache-Control", "public, max-age=600")
	c.Data(http.StatusOK, "text/html; charset=utf-8", ws.indexPageData)
}

func (ws *StaticWebService) Serve(c *gin.Context) {
	if strings.HasPrefix(c.Request.RequestURI, "/api") {
		c.Next()
	} else if _, err := ws.httpFS.Open(c.Request.URL.Path); err == nil {
		ws.fileServer.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	} else {
		c.Next()
	}
}

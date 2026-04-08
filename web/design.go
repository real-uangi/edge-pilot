/*
 * Copyright 2025 Uangi. All rights reserved.
 * @author uangi
 * @date 2025/6/17 13:52
 */

// Package web

package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/real-uangi/fxtrategy"
)

const (
	GroupName    string = "web-design"
	ThemeDefault string = "default"
)

type Design interface {
	fxtrategy.Nameable
	GetFS() http.FileSystem
}

type DesignDefault struct {
	fs   http.FileSystem
	name string
}

func (d *DesignDefault) GetFS() http.FileSystem {
	return d.fs
}

func (d *DesignDefault) ItemName() string {
	return d.name
}

//go:embed default/dist/**
var DefaultFS embed.FS

func newDefaultFS() (*DesignDefault, error) {
	subFs, err := fs.Sub(DefaultFS, "default/dist")
	if err != nil {
		return nil, err
	}
	return &DesignDefault{
		fs:   http.FS(subFs),
		name: ThemeDefault,
	}, nil
}

/*
 * Copyright 2026 Uangi. All rights reserved.
 * @author uangi
 * @date 2026/1/22 10:44
 */

// Package web

package web

import (
	"github.com/real-uangi/fxtrategy"
	"go.uber.org/fx"
)

var Module = fx.Module(
	"web",
	fxtrategy.ProvideContext[Design](GroupName),
	fxtrategy.ProvideItem[Design](newDefaultFS, GroupName),

	fx.Provide(newStaticWebService),
)

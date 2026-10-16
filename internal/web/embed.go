// Package web — веб-слой серверного рендеринга: layout-шаблоны (templ) и
// статические ресурсы, встроенные в бинарник через go:embed (htmx, CSS).
package web

import "embed"

// Static — статические ресурсы веб-слоя, встроенные в бинарник.
//
//go:embed static/*
var Static embed.FS

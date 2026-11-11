// Package static embeds application artwork and icon resources.
package static

import "embed"

// FS contains the static artwork shipped with ProxiLens.
//
//go:embed wlm-logo.png icons/*
var FS embed.FS

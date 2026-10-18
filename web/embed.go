package web

import "embed"

// DistFS contains the embedded frontend build artifacts.
//
//go:embed all:dist
var DistFS embed.FS

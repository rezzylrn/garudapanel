// Package web embeds the production frontend assets built by Vite.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

// DistFS returns an fs.FS sub-rooted at the dist folder.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

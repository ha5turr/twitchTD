// Package web embeds the overlay and panel so the server ships as one binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var static embed.FS

// Files returns the static files rooted at the static directory.
func Files() fs.FS {
	sub, err := fs.Sub(static, "static")
	if err != nil {
		panic(err)
	}
	return sub
}

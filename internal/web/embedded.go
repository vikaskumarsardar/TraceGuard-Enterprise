package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// StaticAssets embeds the static Web Dashboard UI files into the compiled Go binary.
//
//go:embed static/*
var staticAssets embed.FS

// GetFileSystem returns an http.FileSystem for serving embedded UI assets.
func GetFileSystem() (http.FileSystem, error) {
	sub, err := fs.Sub(staticAssets, "static")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}

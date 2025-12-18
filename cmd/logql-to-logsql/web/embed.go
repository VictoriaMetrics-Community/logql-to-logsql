package web

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"path"
)

//go:embed dist/*
var dist embed.FS

// DistFS returns an http.FileSystem for the embedded frontend bundle.
func DistFS() http.FileSystem {
	fsys, err := fs.Sub(dist, "dist")
	if err != nil {
		log.Printf("FATAL: failed to create embedded filesystem: %v", err)
		log.Fatal("This is likely a build issue - ensure 'dist' directory exists")
	}
	return http.FS(fsys)
}

// ReadFile reads a file from the embedded dist folder.
func ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(dist, path.Join("dist", name))
}

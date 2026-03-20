package server

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var embeddedFS embed.FS

// EmbeddedStaticFS returns the embedded React build files as an fs.FS.
func EmbeddedStaticFS() fs.FS {
	sub, err := fs.Sub(embeddedFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

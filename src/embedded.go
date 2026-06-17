package main

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed embed/schema.sql embed/favicon.ico embed/ini embed/lua embed/webapp
var embeddedAssets embed.FS

func assetPath(filePath string) string {
	cleaned := path.Clean("/" + strings.TrimPrefix(filepath.ToSlash(filePath), "/"))
	return path.Join("embed", strings.TrimPrefix(cleaned, "/"))
}

func assetExists(filePath string) bool {
	_, err := fs.Stat(embeddedAssets, assetPath(filePath))
	return err == nil
}

func assetSubFS(dir string) http.FileSystem {
	sub, err := fs.Sub(embeddedAssets, path.Join("embed", dir))
	if err != nil {
		return http.FS(embeddedAssets)
	}
	return http.FS(sub)
}

func OpenAsset(filePath string) (io.ReadCloser, error) {
	if debug {
		return os.Open(filepath.FromSlash(assetPath(filePath)))
	}

	return embeddedAssets.Open(assetPath(filePath))
}

func StaticAssetsFS() http.FileSystem {
	if debug {
		return http.Dir("embed")
	}
	return assetSubFS("")
}

func SpaAssetsFS() http.FileSystem {
	return assetSubFS("webapp/dist")
}

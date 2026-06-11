package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// routeSpa serves the Vue SPA (webapp/dist) for every non-API GET. Unknown
// paths fall back to index.html so vue-router history mode survives reloads.
func routeSpa(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api") {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{"code": "not_found", "message": "Unknown API endpoint"},
		})
		return
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Status(http.StatusNotFound)
		return
	}

	p := path.Clean(c.Request.URL.Path)
	if p == "/" {
		p = "/index.html"
	}

	if debug {
		base := filepath.Join("..", "webapp", "dist")
		full := filepath.Join(base, filepath.FromSlash(p))
		if st, err := os.Stat(full); err != nil || st.IsDir() {
			full = filepath.Join(base, "index.html")
		}
		c.File(full)
		return
	}

	asset := "/webapp/dist" + p
	if FindFile(asset) == nil {
		asset = "/webapp/dist/index.html"
	}
	c.FileFromFS(asset, Assets)
}

// routeLegacyApp redirects pre-cutover /app/* bookmarks to the new root.
func routeLegacyApp(c *gin.Context) {
	target := strings.TrimPrefix(c.Request.URL.Path, "/app")
	if target == "" {
		target = "/"
	}
	c.Redirect(http.StatusMovedPermanently, target)
}

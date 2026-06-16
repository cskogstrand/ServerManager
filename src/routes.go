package main

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// routeSpa serves the Vue SPA for every non-API GET. Unknown
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
		base := filepath.Join("embed", "webapp", "dist")
		full := filepath.Join(base, filepath.FromSlash(p))
		if st, err := os.Stat(full); err != nil || st.IsDir() {
			full = filepath.Join(base, "index.html")
		}
		// The SPA shell must always revalidate so a new build is picked up;
		// hashed /assets/* are immutable and keep their default caching.
		if strings.HasSuffix(full, "index.html") {
			c.Header("Cache-Control", "no-cache")
		}
		c.File(full)
		return
	}

	// Serve embedded bytes directly. Using http.FileServer here (c.FileFromFS)
	// would 301-redirect "/index.html" -> "./", breaking every SPA route.
	asset := "/webapp/dist" + p
	if !assetExists(asset) {
		asset = "/webapp/dist/index.html"
	}
	if strings.HasSuffix(asset, "index.html") {
		c.Header("Cache-Control", "no-cache")
	}
	f, err := OpenAsset(asset)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	ctype := mime.TypeByExtension(path.Ext(asset))
	if ctype == "" {
		ctype = http.DetectContentType(data)
	}
	c.Data(http.StatusOK, ctype, data)
}

// routeLegacyApp redirects pre-cutover /app/* bookmarks to the new root.
func routeLegacyApp(c *gin.Context) {
	target := strings.TrimPrefix(c.Request.URL.Path, "/app")
	if target == "" {
		target = "/"
	}
	c.Redirect(http.StatusMovedPermanently, target)
}

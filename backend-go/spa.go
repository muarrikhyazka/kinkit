package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SPAHandler serves the built React app, falling back to index.html for
// any path that isn't a real file on disk (client-side routing).
//
// Caching (PWA-friendly):
//   - /assets/* are content-hashed by Vite -> cached for a year, immutable
//   - sw.js, index.html, manifest -> always revalidated, so a deploy is
//     picked up on the next load (a stale service worker would pin old code)
func SPAHandler(staticDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean("/" + r.URL.Path)
		path := filepath.Join(staticDir, clean)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			setStaticHeaders(w, clean)
			http.ServeFile(w, r, path)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	}
}

func setStaticHeaders(w http.ResponseWriter, urlPath string) {
	h := w.Header()
	switch {
	case urlPath == "/sw.js":
		h.Set("Content-Type", "text/javascript; charset=utf-8")
		h.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		h.Set("Service-Worker-Allowed", "/")
	case strings.HasSuffix(urlPath, ".webmanifest"):
		h.Set("Content-Type", "application/manifest+json")
		h.Set("Cache-Control", "no-cache")
	case strings.HasPrefix(urlPath, "/assets/"):
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	case urlPath == "/index.html":
		h.Set("Cache-Control", "no-cache")
	default:
		h.Set("Cache-Control", "public, max-age=86400")
	}
}

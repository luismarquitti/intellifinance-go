package http

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/luismarquitti/intellifinance-go/web"
)

// RegisterSPARoutes mounts the embedded frontend Single Page Application (SPA) on the router.
// It serves static files from web.DistFS and falls back to index.html for client-side routing.
func RegisterSPARoutes(r *chi.Mux) {
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	indexBytes, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		indexBytes = []byte("<!DOCTYPE html><html><body><h1>IntelliFinance</h1></body></html>")
	}

	serveIndex := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexBytes)
	}

	// Root path serves index.html
	r.Get("/", serveIndex)

	// Catch-all for assets and client-side SPA routes
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		path := strings.TrimPrefix(req.URL.Path, "/")

		// If it's an API route or health check, return 404
		if strings.HasPrefix(req.URL.Path, "/api/") ||
			strings.HasPrefix(req.URL.Path, "/healthz") ||
			strings.HasPrefix(req.URL.Path, "/readyz") {
			http.NotFound(w, req)
			return
		}

		// Check if file exists in dist
		if path != "" {
			if f, err := distFS.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, req)
				return
			}
		}

		// Fallback to index.html for client-side routing
		serveIndex(w, req)
	})
}

// Package webui serves the embedded React single-page application.
//
// At build time the Docker image populates the static/ directory with the
// compiled Vite output (see apps/api/Dockerfile). A placeholder index.html is
// committed so local `go build` / `make api-dev` compile without a web build.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:static
var embedded embed.FS

// Handler returns an http.Handler that serves the embedded SPA. Requests that
// map to a real asset are served directly; every other path falls back to
// index.html so client-side routing works.
func Handler() http.Handler {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic("webui: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic("webui: missing index.html: " + err.Error())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if info, statErr := fs.Stat(sub, name); name != "" && statErr == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve the application shell for unknown routes.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

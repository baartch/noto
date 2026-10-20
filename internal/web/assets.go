package web

import (
	"bytes"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

var (
	staticFS = func() fs.FS {
		assets, err := fs.Sub(staticFiles, "static")
		if err != nil {
			panic(err)
		}
		return assets
	}()
)

func serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeMethodError(w, "GET, HEAD")
		return
	}
	assetPath := strings.TrimPrefix(r.URL.Path, "/")
	if assetPath == "" {
		assetPath = "index.html"
	}
	if !fs.ValidPath(assetPath) {
		http.NotFound(w, r)
		return
	}
	info, err := fs.Stat(staticFS, assetPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(staticFS, assetPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	if contentType := mime.TypeByExtension(filepath.Ext(assetPath)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), bytes.NewReader(data))
}

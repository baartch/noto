package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestEmbeddedAssetsAvailable(t *testing.T) {
	h := NewHandler(NewService(nil, nil, nil, "", true), testOrigin)
	for path, wantType := range map[string]string{
		"/":          "text/html",
		"/app.js":    "javascript",
		"/app.css":   "text/css",
		"/theme.css": "text/css",
	} {
		recorder := request(h, http.MethodGet, path, "", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200; headers=%v body=%q", path, recorder.Code, recorder.Header(), recorder.Body.String())
		}
		if !strings.Contains(recorder.Header().Get("Content-Type"), wantType) {
			t.Errorf("GET %s Content-Type = %q, want %q", path, recorder.Header().Get("Content-Type"), wantType)
		}
		if strings.Contains(strings.ToLower(recorder.Body.String()), "src=\"http") || strings.Contains(strings.ToLower(recorder.Body.String()), "href=\"http") || strings.Contains(strings.ToLower(recorder.Body.String()), "url(http") {
			t.Errorf("GET %s includes an external asset URL", path)
		}
	}
	if recorder := request(h, http.MethodGet, "/missing.js", "", ""); recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown asset status = %d, want 404", recorder.Code)
	}
}

package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestEmbeddedAssetsAvailable(t *testing.T) {
	h := NewHandler(NewService(nil, nil, nil, "", true), testOrigin)
	bodies := make(map[string]string)
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
		bodies[path] = recorder.Body.String()
	}

	for _, want := range []string{`id="settings-panel"`, `role="dialog"`, `id="workspace"`, `id="chat-panel"`, `id="message-input"`, `id="memory-panel"`, `id="memory-tabs"`} {
		if !strings.Contains(bodies["/"], want) {
			t.Errorf("embedded index.html is missing required web UI entry point %q", want)
		}
	}
	externalResource := regexp.MustCompile(`(?i)(?:src|href)\s*=\s*["']\s*https?://|url\(\s*["']?\s*https?://|@import\s+(?:url\(\s*)?["']?\s*https?://`)
	for _, path := range []string{"/", "/app.css", "/theme.css"} {
		if externalResource.MatchString(bodies[path]) {
			t.Errorf("GET %s contains an external script, stylesheet, font, or CSS URL", path)
		}
	}
	if recorder := request(h, http.MethodGet, "/missing.js", "", ""); recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown asset status = %d, want 404", recorder.Code)
	}
}

func TestEmbeddedBrowserBundleIsLocalBunIIFE(t *testing.T) {
	manifest, err := os.ReadFile("ui/package.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), `"esbuild"`) {
		t.Fatal("web UI package still depends on esbuild")
	}

	bundle, err := staticFS.Open("app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer bundle.Close()
	data, err := io.ReadAll(bundle)
	if err != nil {
		t.Fatal(err)
	}
	code := strings.TrimSpace(string(data))
	if !strings.HasPrefix(code, "(()=>{") {
		t.Fatal("embedded app.js is not a classic Bun IIFE bundle")
	}
	if strings.Contains(code, `from"preline`) || strings.Contains(code, `from"lucide`) {
		t.Fatal("embedded app.js contains unresolved package imports")
	}
	if !strings.Contains(code, "data-hs-overlay") {
		t.Fatal("embedded app.js does not include the Preline overlay runtime")
	}
}

func TestBrowserFixtureUsesProductionCSP(t *testing.T) {
	fixture, err := os.ReadFile("ui/browser/fixture-server.js")
	if err != nil {
		t.Fatal(err)
	}
	const marker = `"Content-Security-Policy": "`
	start := strings.Index(string(fixture), marker)
	if start < 0 {
		t.Fatal("browser fixture has no Content-Security-Policy header")
	}
	policy := string(fixture)[start+len(marker):]
	end := strings.IndexByte(policy, '"')
	if end < 0 {
		t.Fatal("browser fixture CSP value is not terminated")
	}

	recorder := httptest.NewRecorder()
	setSecurityHeaders(recorder)
	if got := recorder.Header().Get("Content-Security-Policy"); got != policy[:end] {
		t.Fatalf("browser fixture CSP = %q, production CSP = %q", policy[:end], got)
	}
}

func TestBrowserFixtureMicrophonePolicyMatchesProduction(t *testing.T) {
	fixture, err := os.ReadFile("ui/browser/fixture-server.js")
	if err != nil {
		t.Fatal(err)
	}
	const marker = `"Permissions-Policy": "`
	start := strings.Index(string(fixture), marker)
	if start < 0 {
		t.Fatal("browser fixture has no Permissions-Policy header")
	}
	policy := string(fixture)[start+len(marker):]
	end := strings.IndexByte(policy, '"')
	if end < 0 {
		t.Fatal("browser fixture Permissions-Policy value is not terminated")
	}
	if got, want := policy[:end], "camera=(), microphone=(self), geolocation=()"; got != want {
		t.Fatalf("browser fixture Permissions-Policy = %q, want %q", got, want)
	}

	recorder := httptest.NewRecorder()
	setSecurityHeaders(recorder)
	if got := recorder.Header().Get("Permissions-Policy"); got != policy[:end] {
		t.Fatalf("browser fixture Permissions-Policy = %q, production Permissions-Policy = %q", policy[:end], got)
	}
}

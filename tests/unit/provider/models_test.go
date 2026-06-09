package provider_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"noto/internal/provider"
)

func TestListModels_ParsesContextLengthAndToolSupport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"openrouter/model-a","owned_by":"openrouter","context_length":128000,"supported_parameters":["tools","max_tokens"]}]}`))
	}))
	defer ts.Close()

	models, err := provider.ListModels(context.Background(), provider.Config{Endpoint: ts.URL})
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].ContextLength != 128000 {
		t.Fatalf("context length = %d, want 128000", models[0].ContextLength)
	}
	if !models[0].ToolSupport.SupportsTools {
		t.Fatalf("expected tool support to be true")
	}
}

func TestListModels_ParsesMissingToolSupportAsFalse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"model-b","owned_by":"provider","context_length":8192,"supported_parameters":["max_tokens"]}]}`))
	}))
	defer ts.Close()

	models, err := provider.ListModels(context.Background(), provider.Config{Endpoint: ts.URL})
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].ToolSupport.SupportsTools {
		t.Fatalf("expected tool support to be false")
	}
}

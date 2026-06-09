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
		_, _ = w.Write([]byte(`{"data":[{"id":"openrouter/model-a","owned_by":"openrouter","context_length":128000,"top_provider":{"context_length":64000},"pricing":{"prompt":"0.000001","completion":"0.000002","request":"0","image":"0","web_search":"0","internal_reasoning":"0","input_cache_read":"0","input_cache_write":"0"},"supported_parameters":["tools","max_tokens"]}]}`))
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
	if models[0].TopProviderContext != 64000 {
		t.Fatalf("top provider context = %d, want 64000", models[0].TopProviderContext)
	}
	if models[0].Pricing.Prompt != "0.000001" {
		t.Fatalf("prompt pricing = %q, want 0.000001", models[0].Pricing.Prompt)
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

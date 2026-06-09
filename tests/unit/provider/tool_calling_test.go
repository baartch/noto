package provider_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"noto/internal/provider"
)

func TestOpenAICompatible_ParsesToolCalls(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"model":"openrouter/test-model",
			"output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"search_memory_keywords","arguments":"{\"query\":\"launch\"}"}],
			"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}
		}`))
	}))
	defer ts.Close()

	adapter := provider.NewOpenAICompatible(provider.Config{Endpoint: ts.URL})
	resp, err := adapter.Complete(context.Background(), provider.CompletionRequest{Model: "openrouter/test-model"})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "search_memory_keywords" {
		t.Fatalf("unexpected tool name: %s", resp.ToolCalls[0].Name)
	}
}

func TestOpenAICompatible_SendsToolsInRequest(t *testing.T) {
	var body string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		_, _ = w.Write([]byte(`{"model":"openrouter/test-model","output":[{"content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}`))
	}))
	defer ts.Close()

	adapter := provider.NewOpenAICompatible(provider.Config{Endpoint: ts.URL})
	_, err := adapter.Complete(context.Background(), provider.CompletionRequest{
		Model: "openrouter/test-model",
		Tools: []provider.ToolDefinition{{Type: "function", Name: "search_memory_keywords", Description: "Search memory", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if !strings.Contains(body, `"tools"`) || !strings.Contains(body, `"search_memory_keywords"`) {
		t.Fatalf("expected tools payload in request body, got: %s", body)
	}
}

func TestOpenAICompatible_InvalidToolPayloadStillErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer ts.Close()

	adapter := provider.NewOpenAICompatible(provider.Config{Endpoint: ts.URL})
	if _, err := adapter.Complete(context.Background(), provider.CompletionRequest{Model: "openrouter/test-model"}); err == nil {
		t.Fatalf("expected decode error")
	}
}

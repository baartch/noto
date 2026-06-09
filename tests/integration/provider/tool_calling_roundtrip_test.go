package integration

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"noto/internal/provider"
)

func TestToolCallingRoundTrip_RequestIncludesToolsAndFollowupCarriesToolResult(t *testing.T) {
	bodies := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		if len(bodies) == 1 {
			_, _ = w.Write([]byte(`{"model":"openrouter/test-model","output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"search_memory_keywords","arguments":"{\"query\":\"launch\"}"}],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}`))
			return
		}
		_, _ = w.Write([]byte(`{"model":"openrouter/test-model","output":[{"content":[{"type":"output_text","text":"final answer"}]}],"usage":{"input_tokens":20,"output_tokens":5,"total_tokens":25}}`))
	}))
	defer server.Close()

	adapter := provider.NewOpenAICompatible(provider.Config{Endpoint: server.URL, Model: "openrouter/test-model"})
	first, err := adapter.Complete(context.Background(), provider.CompletionRequest{Model: "openrouter/test-model", Tools: []provider.ToolDefinition{{Type: "function", Name: "search_memory_keywords", Parameters: map[string]any{"type": "object"}}}})
	if err != nil {
		t.Fatalf("first complete: %v", err)
	}
	if len(first.ToolCalls) != 1 {
		t.Fatalf("expected tool call, got %d", len(first.ToolCalls))
	}
	_, err = adapter.Complete(context.Background(), provider.CompletionRequest{Model: "openrouter/test-model", Tools: []provider.ToolDefinition{{Type: "function", Name: "search_memory_keywords", Parameters: map[string]any{"type": "object"}}}, Messages: []provider.Message{{Role: "assistant", Content: "tool call", ToolCallID: first.ToolCalls[0].CallID}, {Role: "tool", Content: `[{"record_id":"n1"}]`, ToolCallID: first.ToolCalls[0].CallID}}})
	if err != nil {
		t.Fatalf("follow-up complete: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 provider calls, got %d", len(bodies))
	}
	if !strings.Contains(bodies[0], `"tools"`) || !strings.Contains(bodies[1], `"function_call_output"`) || strings.Contains(bodies[1], `"role":"tool"`) {
		t.Fatalf("unexpected request bodies: %#v", bodies)
	}
}

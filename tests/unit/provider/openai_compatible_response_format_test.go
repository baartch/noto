package provider_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"noto/internal/provider"
)

func TestCompleteMarshalsResponseFormat(t *testing.T) {
	var captured []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		captured = data
		_, _ = w.Write([]byte(`{"model":"test-model","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer ts.Close()

	adapter := provider.NewOpenAICompatible(provider.Config{Endpoint: ts.URL})
	_, err := adapter.Complete(context.Background(), provider.CompletionRequest{
		Model: "test-model",
		ResponseFormat: &provider.ResponseFormat{
			Name:   "memory_extraction",
			Strict: true,
			Schema: map[string]any{"type": "object"},
		},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatalf("unmarshal captured body: %v: %s", err, captured)
	}
	rf, ok := body["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("no response_format in body: %s", captured)
	}
	if rf["type"] != "json_schema" {
		t.Fatalf("response_format.type = %v, want json_schema", rf["type"])
	}
	js, ok := rf["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("no response_format.json_schema in body: %s", captured)
	}
	if js["name"] != "memory_extraction" {
		t.Fatalf("json_schema.name = %v, want memory_extraction", js["name"])
	}
	strict, ok := js["strict"].(bool)
	if !ok || !strict {
		t.Fatalf("json_schema.strict = %v, want true", js["strict"])
	}
	if _, present := body["provider"]; present {
		t.Fatalf("provider prefs must not be sent to non-OpenRouter endpoints: %s", captured)
	}
}

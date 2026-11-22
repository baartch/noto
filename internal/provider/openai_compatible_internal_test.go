package provider

import "testing"

func TestIsOpenRouterEndpoint(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		want     bool
	}{
		{endpoint: "https://openrouter.ai/api/v1", want: true},
		{endpoint: "https://OpenRouter.AI/api/v1", want: true},
		{endpoint: "https://openrouter.ai:8443/api/v1", want: true},
		{endpoint: "https://api.openai.com/v1", want: false},
		{endpoint: "https://openrouter.ai.evil/v1", want: false},
		{endpoint: "https://evil-openrouter.ai/v1", want: false},
		{endpoint: "https://evil.com/?next=openrouter.ai", want: false},
		{endpoint: "https://openrouter.ai@evil.com/v1", want: false},
		{endpoint: "http://openrouter.ai/api/v1", want: false},
		{endpoint: "", want: false},
	} {
		t.Run(tc.endpoint, func(t *testing.T) {
			if got := IsOpenRouterEndpoint(tc.endpoint); got != tc.want {
				t.Fatalf("IsOpenRouterEndpoint(%q) = %t, want %t", tc.endpoint, got, tc.want)
			}
		})
	}
}

func TestBuildChatCompletionsPayload_ResponseFormat(t *testing.T) {
	req := CompletionRequest{Messages: []Message{{Role: "user", Content: "hi"}},
		ResponseFormat: &ResponseFormat{Name: "test_schema", Strict: true, Schema: map[string]any{"type": "object"}}}
	payload := buildChatCompletionsPayload(Config{Endpoint: "https://api.openai.com/v1"}, req)
	if payload.ResponseFormat == nil || payload.ProviderPrefs != nil {
		t.Fatalf("want response_format without provider prefs, got %+v / %+v", payload.ResponseFormat, payload.ProviderPrefs)
	}
	if payload.ResponseFormat.JSONSchema.Name != "test_schema" || !payload.ResponseFormat.JSONSchema.Strict {
		t.Fatalf("schema name/strict mismatch: %+v", payload.ResponseFormat.JSONSchema)
	}
	if payload.ResponseFormat.Type != "json_schema" {
		t.Fatalf("type = %q, want json_schema", payload.ResponseFormat.Type)
	}
}

func TestBuildChatCompletionsPayload_NoProviderPrefsForNonOpenRouter(t *testing.T) {
	req := CompletionRequest{ResponseFormat: &ResponseFormat{Name: "s", Strict: true, Schema: map[string]any{}}}
	payload := buildChatCompletionsPayload(Config{Endpoint: "https://api.example.com/v1"}, req)
	if payload.ProviderPrefs != nil {
		t.Fatal("provider prefs must not be set for non-OpenRouter endpoints")
	}
}

func TestBuildChatCompletionsPayload_OpenRouterRequiresParameters(t *testing.T) {
	req := CompletionRequest{ResponseFormat: &ResponseFormat{Name: "s", Strict: true, Schema: map[string]any{}}}
	payload := buildChatCompletionsPayload(Config{Endpoint: "https://openrouter.ai/api/v1"}, req)
	if payload.ProviderPrefs == nil || !payload.ProviderPrefs.RequireParameters {
		t.Fatal("want require_parameters=true on OpenRouter")
	}
	// Without ResponseFormat, even OpenRouter gets no prefs:
	payload = buildChatCompletionsPayload(Config{Endpoint: "https://openrouter.ai/api/v1"}, CompletionRequest{})
	if payload.ProviderPrefs != nil {
		t.Fatal("provider prefs sent without response_format")
	}
}

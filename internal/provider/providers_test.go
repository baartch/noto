package provider

import "testing"

func TestAvailableProviders_NotEmpty(t *testing.T) {
	if len(AvailableProviders) == 0 {
		t.Fatal("AvailableProviders must not be empty")
	}
}

func TestAvailableProviders_OpenRouterEndpoint(t *testing.T) {
	for _, p := range AvailableProviders {
		if p.Name == "OpenRouter" {
			if p.Endpoint != "https://openrouter.ai/api/v1" {
				t.Fatalf("OpenRouter endpoint mismatch: got %q", p.Endpoint)
			}
			return
		}
	}
	t.Fatal("OpenRouter provider not found")
}

func TestAvailableProviders_OpenAIEndpoint(t *testing.T) {
	for _, p := range AvailableProviders {
		if p.Name == "OpenAI" {
			if p.Endpoint != "https://api.openai.com/v1" {
				t.Fatalf("OpenAI endpoint mismatch: got %q", p.Endpoint)
			}
			return
		}
	}
	t.Fatal("OpenAI provider not found")
}

func TestAvailableProviders_AllHaveNamesAndEndpoints(t *testing.T) {
	for i, p := range AvailableProviders {
		if p.Name == "" {
			t.Fatalf("provider %d has empty name", i)
		}
		if p.Endpoint == "" {
			t.Fatalf("provider %d (%s) has empty endpoint", i, p.Name)
		}
	}
}

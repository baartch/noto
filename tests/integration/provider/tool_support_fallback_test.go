package integration

import (
	"context"
	"testing"

	"noto/internal/provider"
)

type noToolAdapter struct{}

func (noToolAdapter) Complete(_ context.Context, _ provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return &provider.CompletionResponse{Content: "basic chat", Model: "test"}, nil
}
func (noToolAdapter) Embed(_ context.Context, _ provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	return &provider.EmbeddingResponse{}, nil
}
func (noToolAdapter) ProviderType() string { return "test" }

func TestToolSupportFallback_ChatStillWorksWithoutTools(t *testing.T) {
	adapter := noToolAdapter{}
	resp, err := adapter.Complete(context.Background(), provider.CompletionRequest{Messages: []provider.Message{{Role: "user", Content: "hello"}}})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if resp.Content != "basic chat" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
}

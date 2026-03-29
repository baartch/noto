package provider

import (
	"context"
	"errors"
)

// ErrProviderUnavailable is returned when the provider cannot be reached.
var ErrProviderUnavailable = errors.New("provider: service unavailable")

// ErrInvalidCredentials is returned when authentication fails.
var ErrInvalidCredentials = errors.New("provider: invalid credentials")

// Message represents a single turn in a chat completion request.
type Message struct {
	Role    string
	Content string
}

// CompletionRequest is the normalized request payload sent to a provider.
type CompletionRequest struct {
	Messages    []Message
	Model       string
	MaxTokens   int
	Temperature float64
}

// CompletionResponse is the normalized response from a provider.
type CompletionResponse struct {
	Content          string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	// EstimatedCostUSD is a rough cost estimate based on known model pricing.
	// Zero if the model is not in the pricing table.
	EstimatedCostUSD float64
	// ContextMax is the model's context window size (tokens). Zero if unknown.
	ContextMax int
}

// EmbeddingRequest is the normalized request payload sent to a provider for embeddings.
type EmbeddingRequest struct {
	Input string
	Model string
}

// EmbeddingResponse is the normalized response from a provider for embeddings.
type EmbeddingResponse struct {
	Embedding []float32
	Model     string
}

// Adapter is the interface all provider implementations must satisfy.
type Adapter interface {
	// Complete sends a chat completion request and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Embed sends an embedding request and returns the embedding response.
	Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)

	// ProviderType returns the canonical provider type string (e.g. "openai_compatible").
	ProviderType() string
}

// Config holds the configuration needed to initialize a provider adapter.
type Config struct {
	ProviderType string
	Endpoint     string
	Model        string
	APIKey       string // decrypted at runtime; never persisted in plain text
}

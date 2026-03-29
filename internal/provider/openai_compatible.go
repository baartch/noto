package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultTimeout = 30 * time.Second

// OpenAICompatible implements the Adapter interface for OpenAI-compatible APIs.
type OpenAICompatible struct {
	cfg    Config
	client *http.Client
}

// NewOpenAICompatible creates an OpenAICompatible adapter with the given config.
func NewOpenAICompatible(cfg Config) *OpenAICompatible {
	return &OpenAICompatible{
		cfg: cfg,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// ProviderType returns the canonical provider type identifier.
func (a *OpenAICompatible) ProviderType() string { return "openai_compatible" }

// SetModel updates the default model used when the request has no model set.
func (a *OpenAICompatible) SetModel(model string) { a.cfg.Model = model }

// Complete performs an OpenAI-compatible chat completion request.
func (a *OpenAICompatible) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	endpoint := a.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/chat/completions"
	}

	model := req.Model
	if model == "" {
		model = a.cfg.Model
	}
	payload := openAIRequest{
		Model:       model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	for _, m := range req.Messages {
		payload.Messages = append(payload.Messages, openAIMessage(m))
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("provider: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("provider: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrInvalidCredentials
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider: unexpected status %d: %s", resp.StatusCode, string(data))
	}

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("provider: decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("provider: no choices in response")
	}

	modelName := apiResp.Model
	if modelName == "" {
		modelName = a.cfg.Model
	}
	promptTokens := apiResp.Usage.PromptTokens
	completionTokens := apiResp.Usage.CompletionTokens
	if completionTokens == 0 && apiResp.Usage.TotalTokens > 0 {
		completionTokens = apiResp.Usage.TotalTokens - promptTokens
	}
	totalTokens := apiResp.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	info := modelInfo(modelName)

	return &CompletionResponse{
		Content:          apiResp.Choices[0].Message.Content,
		Model:            modelName,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		EstimatedCostUSD: estimateCost(modelName, promptTokens, completionTokens),
		ContextMax:       info.contextWindow,
	}, nil
}

// ---- wire types (unexported) ------------------------------------------------

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

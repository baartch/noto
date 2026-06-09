package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

// Embed performs an OpenAI-compatible embeddings request.
func (a *OpenAICompatible) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	endpoint := a.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = strings.TrimSuffix(endpoint, "/embeddings")
	endpoint = strings.TrimSuffix(endpoint, "/embeddings/models")
	endpoint = strings.TrimSuffix(endpoint, "/chat/completions")
	endpoint = strings.TrimSuffix(endpoint, "/completions")
	endpoint = strings.TrimSuffix(endpoint, "/responses")
	endpoint = strings.TrimRight(endpoint, "/")
	endpoint += "/embeddings"

	model := req.Model
	if model == "" {
		model = a.cfg.Model
	}

	payload := openAIEmbeddingRequest{
		Model: model,
		Input: req.Input,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("provider: marshal embedding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("provider: create embedding request: %w", err)
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("provider: read embedding response: %w", err)
	}
	if os.Getenv("DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "embeddings response status=%d body=%s\n", resp.StatusCode, string(bodyBytes))
	}
	var apiResp openAIEmbeddingResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("provider: decode embedding response: %w: %s", err, string(bodyBytes))
	}
	if len(apiResp.Data) == 0 {
		return nil, errors.New("provider: no embedding data in response")
	}

	vector := make([]float32, len(apiResp.Data[0].Embedding))
	for i, v := range apiResp.Data[0].Embedding {
		vector[i] = float32(v)
	}
	modelName := apiResp.Model
	if modelName == "" {
		modelName = model
	}

	usage := Usage{HasUsage: false}
	promptTokens := apiResp.Usage.InputTokens
	if promptTokens == 0 {
		promptTokens = apiResp.Usage.PromptTokens
	}
	if apiResp.Usage.TotalTokens > 0 || apiResp.Usage.PromptTokensDetails.CachedTokens > 0 || apiResp.Usage.PromptTokensDetails.CacheWriteTokens > 0 || apiResp.Usage.Cost > 0 {
		usage = Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: apiResp.Usage.OutputTokens,
			CachedTokens:     apiResp.Usage.PromptTokensDetails.CachedTokens,
			CacheWriteTokens: apiResp.Usage.PromptTokensDetails.CacheWriteTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
			Cost:             apiResp.Usage.Cost,
			HasUsage:         true,
		}
		if err := ValidateUsage(usage); err != nil {
			usage = Usage{}
		}
	}

	return &EmbeddingResponse{
		Embedding: vector,
		Model:     modelName,
		Usage:     usage,
	}, nil
}

// SetModel updates the default model used when the request has no model set.
func (a *OpenAICompatible) SetModel(model string) { a.cfg.Model = model }

// Complete performs an OpenAI-compatible chat completion request.
func (a *OpenAICompatible) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	endpoint := a.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = strings.TrimSuffix(endpoint, "/chat/completions")
	endpoint = strings.TrimSuffix(endpoint, "/completions")
	endpoint = strings.TrimSuffix(endpoint, "/responses")
	endpoint = strings.TrimSuffix(endpoint, "/embeddings")
	endpoint = strings.TrimSuffix(endpoint, "/embeddings/models")
	endpoint = strings.TrimRight(endpoint, "/")
	endpoint += "/chat/completions"

	model := req.Model
	if model == "" {
		model = a.cfg.Model
	}
	payload := openAIChatCompletionsRequest{
		Model:       model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}
	for _, tool := range req.Tools {
		payload.Tools = append(payload.Tools, openAIChatCompletionsToolDefinition{
			Type: "function",
			Function: openAIChatCompletionsFunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	for _, m := range req.Messages {
		switch {
		case m.Role == "assistant" && m.ToolCallID != "":
			payload.Messages = append(payload.Messages, openAIChatCompletionsMessage{
				Role:    "assistant",
				Content: nil,
				ToolCalls: []openAIChatCompletionsToolCall{{
					ID:   m.ToolCallID,
					Type: "function",
					Function: openAIChatCompletionsFunctionCall{
						Name:      m.ToolCallName,
						Arguments: m.ToolCallArguments,
					},
				}},
			})
		case m.Role == "tool" && m.ToolCallID != "":
			payload.Messages = append(payload.Messages, openAIChatCompletionsMessage{
				Role:       "tool",
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
			})
		default:
			payload.Messages = append(payload.Messages, openAIChatCompletionsMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrInvalidCredentials
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider: unexpected status %d: %s", resp.StatusCode, string(data))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("provider: read response: %w", err)
	}
	var apiResp openAIChatCompletionsResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("provider: decode response: %w: %s", err, string(bodyBytes))
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("provider: chat completions error %s: %s", apiResp.Error.Code, apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 {
		return nil, errors.New("provider: no choices in response")
	}
	msg := apiResp.Choices[0].Message
	content := ""
	if s, ok := msg.Content.(string); ok {
		content = s
	}
	toolCalls := make([]ToolCall, 0, len(msg.ToolCalls))
	for _, call := range msg.ToolCalls {
		if call.Type == "function" && call.Function.Name != "" {
			toolCalls = append(toolCalls, ToolCall{ID: call.ID, CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
		}
	}
	if content == "" && len(toolCalls) == 0 {
		return nil, errors.New("provider: no content in response")
	}

	modelName := apiResp.Model
	if modelName == "" {
		modelName = a.cfg.Model
	}
	promptTokens := apiResp.Usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = apiResp.Usage.InputTokens
	}
	completionTokens := apiResp.Usage.CompletionTokens
	if completionTokens == 0 {
		completionTokens = apiResp.Usage.OutputTokens
	}
	totalTokens := apiResp.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	info := modelInfo(modelName)
	usage := Usage{HasUsage: false}
	if totalTokens > 0 || apiResp.Usage.PromptTokensDetails.CachedTokens > 0 || apiResp.Usage.PromptTokensDetails.CacheWriteTokens > 0 || apiResp.Usage.Cost > 0 {
		usage = Usage{PromptTokens: promptTokens, CompletionTokens: completionTokens, CachedTokens: apiResp.Usage.PromptTokensDetails.CachedTokens, CacheWriteTokens: apiResp.Usage.PromptTokensDetails.CacheWriteTokens, TotalTokens: totalTokens, Cost: apiResp.Usage.Cost, HasUsage: true}
		if err := ValidateUsage(usage); err != nil {
			usage = Usage{}
		}
	}

	return &CompletionResponse{Content: content, Model: modelName, PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: totalTokens, ToolCalls: toolCalls, EstimatedCostUSD: estimateCost(modelName, promptTokens, completionTokens), Usage: usage, ContextMax: info.contextWindow}, nil
}

// ---- wire types (unexported) ------------------------------------------------

type openAIChatCompletionsFunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIChatCompletionsToolDefinition struct {
	Type     string                                 `json:"type"`
	Function openAIChatCompletionsFunctionDefinition `json:"function"`
}

type openAIChatCompletionsFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatCompletionsToolCall struct {
	ID       string                            `json:"id,omitempty"`
	Type     string                            `json:"type"`
	Function openAIChatCompletionsFunctionCall `json:"function"`
}

type openAIChatCompletionsMessage struct {
	Role       string                          `json:"role"`
	Content    any                             `json:"content"`
	ToolCalls  []openAIChatCompletionsToolCall `json:"tool_calls,omitempty"`
	ToolCallID string                          `json:"tool_call_id,omitempty"`
}

type openAIChatCompletionsRequest struct {
	Model       string                               `json:"model"`
	Messages    []openAIChatCompletionsMessage       `json:"messages"`
	Tools       []openAIChatCompletionsToolDefinition `json:"tools,omitempty"`
	MaxTokens   int                                  `json:"max_tokens,omitempty"`
	Temperature float64                              `json:"temperature,omitempty"`
}

type openAIResponsesPromptTokensDetails struct {
	CachedTokens     int `json:"cached_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
}

type openAIResponsesUsage struct {
	InputTokens         int                                `json:"input_tokens"`
	PromptTokens        int                                `json:"prompt_tokens"`
	OutputTokens        int                                `json:"output_tokens"`
	CompletionTokens    int                                `json:"completion_tokens"`
	TotalTokens         int                                `json:"total_tokens"`
	PromptTokensDetails openAIResponsesPromptTokensDetails `json:"prompt_tokens_details"`
	Cost                float64                            `json:"cost"`
}

type openAIChatCompletionsResponse struct {
	Model string `json:"model"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Choices []struct {
		Message openAIChatCompletionsMessage `json:"message"`
	} `json:"choices"`
	Usage openAIResponsesUsage `json:"usage"`
}

type openAIEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Usage openAIResponsesUsage `json:"usage"`
}


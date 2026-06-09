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
	endpoint += "/responses"

	model := req.Model
	if model == "" {
		model = a.cfg.Model
	}
	payload := openAIResponsesRequest{
		Model:           model,
		MaxOutputTokens: req.MaxTokens,
		Temperature:     req.Temperature,
	}
	for _, tool := range req.Tools {
		payload.Tools = append(payload.Tools, openAIResponsesToolDefinition{
			Type:        tool.Type,
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}
	for _, m := range req.Messages {
		msg := openAIResponsesMessage{Role: m.Role}
		if m.ToolCallID != "" {
			msg.CallID = m.ToolCallID
			msg.Content = []openAIResponsesContent{{Type: "function_call_output", Text: m.Content}}
		} else {
			msg.Content = []openAIResponsesContent{{Type: "input_text", Text: m.Content}}
		}
		payload.Input = append(payload.Input, msg)
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("provider: read response: %w", err)
	}
	var apiResp openAIResponsesResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("provider: decode response: %w: %s", err, string(bodyBytes))
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("provider: responses error %s: %s", apiResp.Error.Code, apiResp.Error.Message)
	}
	content := extractResponseText(apiResp.Output)
	toolCalls := extractToolCalls(apiResp.Output)
	if content == "" && len(toolCalls) == 0 {
		return nil, errors.New("provider: no content in response")
	}

	modelName := apiResp.Model
	if modelName == "" {
		modelName = a.cfg.Model
	}
	promptTokens := apiResp.Usage.InputTokens
	if promptTokens == 0 {
		promptTokens = apiResp.Usage.PromptTokens
	}
	completionTokens := apiResp.Usage.OutputTokens
	if completionTokens == 0 {
		completionTokens = apiResp.Usage.CompletionTokens
	}
	if completionTokens == 0 && apiResp.Usage.TotalTokens > 0 {
		completionTokens = apiResp.Usage.TotalTokens - promptTokens
	}
	totalTokens := apiResp.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	info := modelInfo(modelName)

	usage := Usage{HasUsage: false}
	if apiResp.Usage.TotalTokens > 0 || apiResp.Usage.PromptTokensDetails.CachedTokens > 0 || apiResp.Usage.PromptTokensDetails.CacheWriteTokens > 0 || apiResp.Usage.Cost > 0 {
		usage = Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			CachedTokens:     apiResp.Usage.PromptTokensDetails.CachedTokens,
			CacheWriteTokens: apiResp.Usage.PromptTokensDetails.CacheWriteTokens,
			TotalTokens:      totalTokens,
			Cost:             apiResp.Usage.Cost,
			HasUsage:         true,
		}
		if err := ValidateUsage(usage); err != nil {
			usage = Usage{}
		}
	}

	return &CompletionResponse{
		Content:          content,
		Model:            modelName,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		ToolCalls:        toolCalls,
		EstimatedCostUSD: estimateCost(modelName, promptTokens, completionTokens),
		Usage:            usage,
		ContextMax:       info.contextWindow,
	}, nil
}

// ---- wire types (unexported) ------------------------------------------------

type openAIResponsesContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type openAIResponsesMessage struct {
	Role    string                   `json:"role,omitempty"`
	CallID  string                   `json:"call_id,omitempty"`
	Type    string                   `json:"type,omitempty"`
	Content []openAIResponsesContent `json:"content,omitempty"`
}

type openAIResponsesToolDefinition struct {
	Type        string         `json:"type,omitempty"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIResponsesRequest struct {
	Model            string                          `json:"model"`
	Input            []openAIResponsesMessage        `json:"input"`
	Tools            []openAIResponsesToolDefinition `json:"tools,omitempty"`
	MaxOutputTokens  int                             `json:"max_output_tokens,omitempty"`
	Temperature      float64                         `json:"temperature,omitempty"`
	TopP             float64                         `json:"top_p,omitempty"`
	FrequencyPenalty float64                         `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64                         `json:"presence_penalty,omitempty"`
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

type openAIResponsesResponse struct {
	Model  string `json:"model"`
	Status string `json:"status"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Output []struct {
		ID        string                   `json:"id,omitempty"`
		CallID    string                   `json:"call_id,omitempty"`
		Name      string                   `json:"name,omitempty"`
		Type      string                   `json:"type,omitempty"`
		Arguments string                   `json:"arguments,omitempty"`
		Content   []openAIResponsesContent `json:"content"`
	} `json:"output"`
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

func extractResponseText(outputs []struct {
	ID        string                   `json:"id,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Type      string                   `json:"type,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
	Content   []openAIResponsesContent `json:"content"`
}) string {
	for _, output := range outputs {
		for _, content := range output.Content {
			if content.Type == "output_text" || content.Type == "text" || content.Type == "input_text" || content.Type == "" {
				if content.Text != "" {
					return content.Text
				}
			}
		}
	}
	return ""
}

func extractToolCalls(outputs []struct {
	ID        string                   `json:"id,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Type      string                   `json:"type,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
	Content   []openAIResponsesContent `json:"content"`
}) []ToolCall {
	calls := make([]ToolCall, 0)
	for _, output := range outputs {
		if output.Type == "function_call" && output.Name != "" {
			calls = append(calls, ToolCall{ID: output.ID, CallID: output.CallID, Name: output.Name, Arguments: output.Arguments})
		}
	}
	return calls
}

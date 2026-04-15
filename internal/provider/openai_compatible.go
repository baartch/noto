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

	return &EmbeddingResponse{
		Embedding: vector,
		Model:     modelName,
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
	for _, m := range req.Messages {
		payload.Input = append(payload.Input, openAIResponsesMessage{
			Role:    m.Role,
			Content: []openAIResponsesContent{{Type: "input_text", Text: m.Content}},
		})
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
	if content == "" {
		return nil, errors.New("provider: no content in response")
	}

	modelName := apiResp.Model
	if modelName == "" {
		modelName = a.cfg.Model
	}
	promptTokens := apiResp.Usage.InputTokens
	completionTokens := apiResp.Usage.OutputTokens
	if completionTokens == 0 && apiResp.Usage.TotalTokens > 0 {
		completionTokens = apiResp.Usage.TotalTokens - promptTokens
	}
	totalTokens := apiResp.Usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	info := modelInfo(modelName)

	return &CompletionResponse{
		Content:          content,
		Model:            modelName,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		EstimatedCostUSD: estimateCost(modelName, promptTokens, completionTokens),
		ContextMax:       info.contextWindow,
	}, nil
}

// ---- wire types (unexported) ------------------------------------------------

type openAIResponsesContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type openAIResponsesMessage struct {
	Role    string                   `json:"role"`
	Content []openAIResponsesContent `json:"content"`
}

type openAIResponsesRequest struct {
	Model            string                   `json:"model"`
	Input            []openAIResponsesMessage `json:"input"`
	MaxOutputTokens  int                      `json:"max_output_tokens,omitempty"`
	Temperature      float64                  `json:"temperature,omitempty"`
	TopP             float64                  `json:"top_p,omitempty"`
	FrequencyPenalty float64                  `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64                  `json:"presence_penalty,omitempty"`
}

type openAIResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type openAIResponsesResponse struct {
	Model  string `json:"model"`
	Status string `json:"status"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Output []struct {
		Content []openAIResponsesContent `json:"content"`
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
}

func extractResponseText(outputs []struct {
	Content []openAIResponsesContent `json:"content"`
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

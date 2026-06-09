package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
)

// ModelInfo holds metadata about a single model returned by the provider.
type ModelInfo struct {
	ID                 string
	OwnedBy            string
	ContextLength      int
	TopProviderContext int
	Pricing            ModelPricing
	ToolSupport        ToolSupport
}

// ListModels fetches available models from the provider's /models endpoint.
func ListModels(ctx context.Context, cfg Config) ([]ModelInfo, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	// Normalise: strip trailing /chat/completions or /completions if present.
	endpoint = strings.TrimSuffix(endpoint, "/chat/completions")
	endpoint = strings.TrimSuffix(endpoint, "/completions")
	endpoint = strings.TrimRight(endpoint, "/")

	url := endpoint + "/models"

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("provider: build models request: %w", err)
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider: fetch models: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider: models endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID                  string   `json:"id"`
			OwnedBy             string   `json:"owned_by"`
			ContextLength       int      `json:"context_length"`
			SupportedParameters []string `json:"supported_parameters"`
			Pricing             struct {
				Prompt            string `json:"prompt"`
				Completion        string `json:"completion"`
				Request           string `json:"request"`
				Image             string `json:"image"`
				WebSearch         string `json:"web_search"`
				InternalReasoning string `json:"internal_reasoning"`
				InputCacheRead    string `json:"input_cache_read"`
				InputCacheWrite   string `json:"input_cache_write"`
			} `json:"pricing"`
			TopProvider struct {
				ContextLength int `json:"context_length"`
			} `json:"top_provider"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("provider: decode models response: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		supportsTools := slices.Contains(m.SupportedParameters, "tools")
		models = append(models, ModelInfo{
			ID:                 m.ID,
			OwnedBy:            m.OwnedBy,
			ContextLength:      m.ContextLength,
			TopProviderContext: m.TopProvider.ContextLength,
			Pricing: ModelPricing{
				Prompt:            m.Pricing.Prompt,
				Completion:        m.Pricing.Completion,
				Request:           m.Pricing.Request,
				Image:             m.Pricing.Image,
				WebSearch:         m.Pricing.WebSearch,
				InternalReasoning: m.Pricing.InternalReasoning,
				InputCacheRead:    m.Pricing.InputCacheRead,
				InputCacheWrite:   m.Pricing.InputCacheWrite,
			},
			ToolSupport: ToolSupport{SupportsTools: supportsTools},
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})
	return models, nil
}

// ListEmbeddingModels fetches available embedding models from /embeddings/models.
func ListEmbeddingModels(ctx context.Context, cfg Config) ([]ModelInfo, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = strings.TrimSuffix(endpoint, "/chat/completions")
	endpoint = strings.TrimSuffix(endpoint, "/completions")
	endpoint = strings.TrimRight(endpoint, "/")

	url := endpoint + "/embeddings/models"

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("provider: build embeddings models request: %w", err)
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider: fetch embeddings models: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider: embeddings models endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("provider: decode embeddings models response: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, ModelInfo{ID: m.ID, OwnedBy: m.OwnedBy})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})
	return models, nil
}

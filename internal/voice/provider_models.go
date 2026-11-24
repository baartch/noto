package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"noto/internal/provider"
)

const transcriptionModelsResponseLimit = 4 << 20

// ErrTranscriptionModelsUnsupported means the active endpoint cannot reliably
// identify speech-to-text models.
var ErrTranscriptionModelsUnsupported = errors.New("voice: transcription model discovery unsupported")

// ListTranscriptionModels returns only models verified by OpenRouter's dedicated
// transcription output-modality metadata.
func ListTranscriptionModels(ctx context.Context, cfg provider.Config) ([]string, error) {
	if !provider.IsOpenRouterEndpoint(cfg.Endpoint) {
		return nil, ErrTranscriptionModelsUnsupported
	}
	return listTranscriptionModels(ctx, cfg, newTranscriptionModelsHTTPClient())
}

func newTranscriptionModelsHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// listTranscriptionModels accepts a client so request and filtering behavior can
// be tested without accessing the public provider.
func listTranscriptionModels(ctx context.Context, cfg provider.Config, client *http.Client) ([]string, error) {
	endpoint, err := normalizeOpenRouterEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/models"
	query := endpoint.Query()
	query.Set("output_modalities", "transcription")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("voice: build transcription models request: %w", err)
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voice: fetch transcription models: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("voice: OpenRouter models endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, transcriptionModelsResponseLimit+1))
	if err != nil {
		return nil, fmt.Errorf("voice: read transcription models response: %w", err)
	}
	if len(body) > transcriptionModelsResponseLimit {
		return nil, fmt.Errorf("voice: transcription models response exceeds %d bytes", transcriptionModelsResponseLimit)
	}
	var result struct {
		Data []struct {
			ID           string `json:"id"`
			Architecture struct {
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("voice: decode transcription models response: %w", err)
	}

	unique := make(map[string]struct{}, len(result.Data))
	for _, model := range result.Data {
		id := strings.TrimSpace(model.ID)
		if id == "" || !containsTranscription(model.Architecture.OutputModalities) {
			continue
		}
		unique[id] = struct{}{}
	}
	models := make([]string, 0, len(unique))
	for id := range unique {
		models = append(models, id)
	}
	sort.Strings(models)
	return models, nil
}

func normalizeOpenRouterEndpoint(endpoint string) (*url.URL, error) {
	u, err := url.Parse(endpoint)
	if err != nil || !provider.IsOpenRouterEndpoint(endpoint) {
		return nil, ErrTranscriptionModelsUnsupported
	}
	path := strings.TrimRight(u.Path, "/")
	path = strings.TrimSuffix(path, "/chat/completions")
	path = strings.TrimSuffix(path, "/completions")
	u.Path = strings.TrimRight(path, "/")
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}

func containsTranscription(modalities []string) bool {
	return slices.Contains(modalities, "transcription")
}

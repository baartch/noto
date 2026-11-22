package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"noto/internal/provider"
)

const (
	providerTranscriptionTimeout  = 60 * time.Second
	providerTranscriptionMaxReply = 1 << 20
)

type providerTranscriber struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

// NewProviderTranscriber creates an OpenAI-compatible speech-to-text adapter.
func NewProviderTranscriber(cfg provider.Config, model string) Transcriber {
	return &providerTranscriber{
		endpoint: cfg.Endpoint,
		model:    model,
		apiKey:   cfg.APIKey,
		client: &http.Client{
			Timeout: providerTranscriptionTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (t *providerTranscriber) Transcribe(ctx context.Context, audio []byte) (string, error) {
	if err := ValidateWAV(audio); err != nil {
		return "", err
	}
	if strings.TrimSpace(t.model) == "" {
		return "", errors.New("voice: transcription model is required")
	}
	endpoint, err := normalizeTranscriptionEndpoint(t.endpoint)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("model", t.model); err != nil {
		return "", errors.New("voice: build transcription form")
	}
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", `form-data; name="file"; filename="audio.wav"`)
	fileHeader.Set("Content-Type", "audio/wav")
	file, err := form.CreatePart(fileHeader)
	if err != nil {
		return "", errors.New("voice: build transcription form")
	}
	if _, err := file.Write(audio); err != nil {
		return "", errors.New("voice: build transcription form")
	}
	if err := form.Close(); err != nil {
		return "", errors.New("voice: finish transcription form")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", errors.New("voice: build transcription request")
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return "", safeTranscriptionRequestError(ctx, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", provider.ErrInvalidCredentials
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("voice: transcription endpoint returned HTTP %d", resp.StatusCode)
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, providerTranscriptionMaxReply+1))
	if err != nil {
		return "", safeTranscriptionRequestError(ctx, err)
	}
	if len(responseBody) > providerTranscriptionMaxReply {
		return "", errors.New("voice: transcription response is too large")
	}
	var result struct {
		Text *string `json:"text"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", errors.New("voice: invalid transcription response")
	}
	if result.Text == nil {
		return "", errors.New("voice: transcription response is missing text")
	}
	return *result.Text, nil
}

func normalizeTranscriptionEndpoint(endpoint string) (string, error) {
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || u.User != nil || (!strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) {
		return "", errors.New("voice: invalid transcription endpoint")
	}
	if strings.EqualFold(u.Scheme, "http") && !isLoopbackHostname(u.Hostname()) {
		return "", errors.New("voice: remote transcription endpoints must use HTTPS")
	}
	path := strings.TrimRight(u.Path, "/")
	for _, suffix := range []string{"/audio/transcriptions", "/chat/completions", "/completions"} {
		path = strings.TrimSuffix(path, suffix)
	}
	u.Path = strings.TrimRight(path, "/") + "/audio/transcriptions"
	u.RawPath = ""
	u.Fragment = ""
	return u.String(), nil
}

func isLoopbackHostname(hostname string) bool {
	if strings.EqualFold(hostname, "localhost") {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

func safeTranscriptionRequestError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("voice: transcription: %w", ctx.Err())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("voice: transcription: %w", context.DeadlineExceeded)
	}
	return provider.ErrProviderUnavailable
}

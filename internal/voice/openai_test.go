package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"noto/internal/provider"
)

func TestProviderTranscriberSendsMultipartWAVAndParsesText(t *testing.T) {
	wav := makePCM16WAV(1, 16000, 16, []byte{1, 2, 3, 4})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/audio/transcriptions" {
			t.Errorf("request = %s %s, want POST /v1/audio/transcriptions", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer stt-secret" {
			t.Errorf("authorization = %q, want bearer credential", got)
		}
		mediaType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(mediaType, "multipart/form-data; boundary=") {
			t.Errorf("content type = %q, want multipart form data", mediaType)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart form: %v", err)
			return
		}
		if got := r.FormValue("model"); got != "provider/stt" {
			t.Errorf("model = %q, want provider/stt", got)
		}
		if got := r.FormValue("language"); got != "" {
			t.Errorf("language = %q, want automatic language detection", got)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("read file part: %v", err)
			return
		}
		defer func() { _ = file.Close() }()
		if header.Filename != "audio.wav" {
			t.Errorf("filename = %q, want audio.wav", header.Filename)
		}
		if got := header.Header.Get("Content-Type"); got != "audio/wav" {
			t.Errorf("audio content type = %q, want audio/wav", got)
		}
		got, err := io.ReadAll(file)
		if err != nil {
			t.Errorf("read audio part: %v", err)
		} else if string(got) != string(wav) {
			t.Errorf("uploaded audio = %d bytes, want exact %d-byte WAV", len(got), len(wav))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"text": "Hallo, Welt."})
	}))
	defer server.Close()

	transcriber := NewProviderTranscriber(provider.Config{
		Endpoint: server.URL + "/v1/chat/completions",
		APIKey:   "stt-secret",
	}, "provider/stt")
	got, err := transcriber.Transcribe(context.Background(), wav)
	if err != nil {
		t.Fatalf("Transcribe(): %v", err)
	}
	if got != "Hallo, Welt." {
		t.Fatalf("transcript = %q, want %q", got, "Hallo, Welt.")
	}
}

func TestProviderTranscriberDoesNotSendInvalidWAV(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	transcriber := NewProviderTranscriber(provider.Config{Endpoint: server.URL}, "provider/stt")
	if _, err := transcriber.Transcribe(context.Background(), []byte("not a WAV")); err == nil {
		t.Fatal("Transcribe() accepted invalid WAV")
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("invalid WAV sent %d HTTP requests, want 0", got)
	}
}

func TestProviderTranscriberSanitizesProviderErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "private audio or credential detail", http.StatusInternalServerError)
	}))
	defer server.Close()

	transcriber := NewProviderTranscriber(provider.Config{Endpoint: server.URL}, "provider/stt")
	_, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("Transcribe() error = %v, want sanitized HTTP status", err)
	}
	if strings.Contains(err.Error(), "private audio or credential detail") {
		t.Fatalf("provider error body leaked: %v", err)
	}
}

func TestProviderTranscriberDoesNotFollowRedirects(t *testing.T) {
	var redirectRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectRequests.Add(1)
		_, _ = io.WriteString(w, `{"text":"unexpected"}`)
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/audio/transcriptions", http.StatusFound)
	}))
	defer redirect.Close()

	transcriber := NewProviderTranscriber(provider.Config{Endpoint: redirect.URL, APIKey: "redirect-secret"}, "provider/stt")
	_, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("Transcribe() error = %v, want HTTP 302", err)
	}
	if got := redirectRequests.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests, want 0", got)
	}
}

func TestProviderTranscriberMapsUnauthorizedResponses(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "private provider detail", status)
			}))
			defer server.Close()

			transcriber := NewProviderTranscriber(provider.Config{Endpoint: server.URL}, "provider/stt")
			_, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
			if !errors.Is(err, provider.ErrInvalidCredentials) {
				t.Fatalf("Transcribe() error = %v, want ErrInvalidCredentials", err)
			}
			if strings.Contains(err.Error(), "private provider detail") {
				t.Fatalf("provider response body leaked: %v", err)
			}
		})
	}
}

func TestProviderTranscriberRejectsMalformedAndOversizedResponses(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"text":"private malformed"`},
		{name: "missing text field", body: `{"result":"ignored"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()

			transcriber := NewProviderTranscriber(provider.Config{Endpoint: server.URL}, "provider/stt")
			_, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
			if err == nil {
				t.Fatal("Transcribe() succeeded for an invalid response")
			}
			if strings.Contains(err.Error(), "private malformed") {
				t.Fatalf("provider response body leaked: %v", err)
			}
		})
	}
}

func TestProviderTranscriberEnforcesResponseSizeLimit(t *testing.T) {
	prefix := `{"text":"`
	suffix := `"}`
	text := strings.Repeat("a", providerTranscriptionMaxReply-len(prefix)-len(suffix))
	validAtLimit := prefix + text + suffix
	if len(validAtLimit) != providerTranscriptionMaxReply {
		t.Fatalf("test response size = %d, want %d", len(validAtLimit), providerTranscriptionMaxReply)
	}

	for _, tc := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "exact limit", body: validAtLimit},
		{name: "one byte over with valid JSON", body: validAtLimit + " ", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()

			transcriber := NewProviderTranscriber(provider.Config{Endpoint: server.URL}, "provider/stt")
			got, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "response is too large") {
					t.Fatalf("Transcribe() error = %v, want response size error", err)
				}
				return
			}
			if err != nil || len(got) != len(text) {
				t.Fatalf("Transcribe() returned text length %d, error %v; want %d characters", len(got), err, len(text))
			}
		})
	}
}

func TestProviderTranscriberReturnsEmptyTranscript(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"text":""}`)
	}))
	defer server.Close()

	transcriber := NewProviderTranscriber(provider.Config{Endpoint: server.URL}, "provider/stt")
	got, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
	if err != nil {
		t.Fatalf("Transcribe() empty text error = %v", err)
	}
	if got != "" {
		t.Fatalf("empty transcript = %q, want empty", got)
	}
}

func TestProviderTranscriberRequiresModel(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	transcriber := NewProviderTranscriber(provider.Config{Endpoint: server.URL}, "  ")
	if _, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2})); err == nil {
		t.Fatal("Transcribe() succeeded without a selected model")
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("missing model sent %d HTTP requests, want 0", got)
	}
}

func TestProviderTranscriberRedactsNetworkErrors(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial %s with %s", req.URL.String(), req.Header.Get("Authorization"))
	})}
	transcriber := &providerTranscriber{
		endpoint: "https://api.example/v1?token=url-secret",
		model:    "provider/stt",
		apiKey:   "header-secret",
		client:   client,
	}
	_, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
	if !errors.Is(err, provider.ErrProviderUnavailable) {
		t.Fatalf("Transcribe() error = %v, want ErrProviderUnavailable", err)
	}
	for _, secret := range []string{"header-secret", "url-secret", "api.example"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("network error leaked %q: %v", secret, err)
		}
	}
}

func TestNormalizeTranscriptionEndpoint(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		want     string
		wantErr  bool
	}{
		{endpoint: "", want: "https://api.openai.com/v1/audio/transcriptions"},
		{endpoint: "https://api.example/v1/", want: "https://api.example/v1/audio/transcriptions"},
		{endpoint: "https://api.example/v1/chat/completions/", want: "https://api.example/v1/audio/transcriptions"},
		{endpoint: "https://api.example/v1/completions", want: "https://api.example/v1/audio/transcriptions"},
		{endpoint: "https://api.example/v1/audio/transcriptions/", want: "https://api.example/v1/audio/transcriptions"},
		{endpoint: "https://api.example/v1?api-version=2026-01", want: "https://api.example/v1/audio/transcriptions?api-version=2026-01"},
		{endpoint: "http://localhost:8080/v1", want: "http://localhost:8080/v1/audio/transcriptions"},
		{endpoint: "http://127.0.0.1:8080/v1", want: "http://127.0.0.1:8080/v1/audio/transcriptions"},
		{endpoint: "http://[::1]:8080/v1", want: "http://[::1]:8080/v1/audio/transcriptions"},
		{endpoint: "http://api.example/v1", wantErr: true},
		{endpoint: "https://user:secret@api.example/v1", wantErr: true},
		{endpoint: "ftp://api.example/v1", wantErr: true},
	} {
		t.Run(tc.endpoint, func(t *testing.T) {
			got, err := normalizeTranscriptionEndpoint(tc.endpoint)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeTranscriptionEndpoint(%q) succeeded, want error", tc.endpoint)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("normalizeTranscriptionEndpoint(%q) = %q, %v; want %q", tc.endpoint, got, err, tc.want)
			}
		})
	}
}

func TestProviderTranscriberHonorsCancellation(t *testing.T) {
	started := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	transcriber := &providerTranscriber{
		endpoint: "https://api.example/v1",
		model:    "provider/stt",
		client:   client,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := transcriber.Transcribe(ctx, makePCM16WAV(1, 16000, 16, []byte{1, 2}))
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("transcription request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Transcribe() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Transcribe() did not return after cancellation")
	}
}

func TestProviderTranscriberHonorsTimeout(t *testing.T) {
	started := make(chan struct{})
	client := &http.Client{
		Timeout: 100 * time.Millisecond,
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			close(started)
			<-r.Context().Done()
			return nil, r.Context().Err()
		}),
	}
	transcriber := &providerTranscriber{
		endpoint: "https://api.example/v1",
		model:    "provider/stt",
		client:   client,
	}
	done := make(chan error, 1)
	go func() {
		_, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("transcription request did not start")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Transcribe() error = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Transcribe() did not return after timeout")
	}
}

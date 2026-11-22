package voice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"noto/internal/provider"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func openRouterTestClient(server *httptest.Server) *http.Client {
	client := newTranscriptionModelsHTTPClient()
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		target, err := url.Parse(server.URL + req.URL.RequestURI())
		if err != nil {
			return nil, err
		}
		forwarded := req.Clone(req.Context())
		forwarded.URL = target
		forwarded.Host = target.Host
		return server.Client().Transport.RoundTrip(forwarded)
	})
	return client
}

func TestListTranscriptionModelsUsesOpenRouterFilterAndOnlyVerifiedModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			t.Errorf("request path = %q, want /api/v1/models", r.URL.Path)
		}
		if got := r.URL.Query().Get("output_modalities"); got != "transcription" {
			t.Errorf("output_modalities = %q, want transcription", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("authorization = %q, want bearer credential", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[
			{"id":"vendor/stt-z","architecture":{"input_modalities":["audio"],"output_modalities":["transcription"]}},
			{"id":"vendor/audio-chat","architecture":{"input_modalities":["audio"],"output_modalities":["text"]}},
			{"id":"vendor/stt-a","architecture":{"input_modalities":["audio"],"output_modalities":["text","transcription"]}},
			{"id":"vendor/stt-a","architecture":{"output_modalities":["transcription"]}},
			{"id":"   ","architecture":{"output_modalities":["transcription"]}},
			{"architecture":{"output_modalities":["transcription"]}},
			{"id":"vendor/unknown","architecture":{"input_modalities":["audio"]}}
		]}`)
	}))
	defer server.Close()

	cfg := provider.Config{
		Endpoint: "https://openrouter.ai/api/v1/chat/completions",
		APIKey:   "secret",
	}
	got, err := listTranscriptionModels(context.Background(), cfg, openRouterTestClient(server))
	if err != nil {
		t.Fatalf("list transcription models: %v", err)
	}
	want := []string{"vendor/stt-a", "vendor/stt-z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %#v, want %#v", got, want)
	}
}

func TestListTranscriptionModelsRejectsNonOpenRouterEndpoint(t *testing.T) {
	for _, endpoint := range []string{
		"",
		"https://api.example.com/v1",
		"https://openrouter.ai.attacker.example/v1",
		"https://attacker-openrouter.ai/v1",
		"https://openrouter.ai.evil/v1",
		"http://openrouter.ai/api/v1",
	} {
		t.Run(endpoint, func(t *testing.T) {
			_, err := ListTranscriptionModels(context.Background(), provider.Config{Endpoint: endpoint})
			if !errors.Is(err, ErrTranscriptionModelsUnsupported) {
				t.Fatalf("error = %v, want unsupported transcription discovery", err)
			}
		})
	}
}

func TestListTranscriptionModelsDoesNotExposeProviderErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "secret response body", http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := provider.Config{Endpoint: "https://openrouter.ai/api/v1"}
	_, err := listTranscriptionModels(context.Background(), cfg, openRouterTestClient(server))
	if err == nil {
		t.Fatal("list transcription models succeeded for a server error")
	}
	if errors.Is(err, ErrTranscriptionModelsUnsupported) {
		t.Fatalf("HTTP failure was misclassified as unsupported discovery: %v", err)
	}
	if strings.Contains(err.Error(), "secret response body") {
		t.Fatalf("provider response body leaked in error: %v", err)
	}
}

func TestNormalizeOpenRouterEndpointStripsCompletionPathAndQuery(t *testing.T) {
	for _, tc := range []struct {
		endpoint string
		wantPath string
		wantHost string
	}{
		{endpoint: "https://openrouter.ai/api/v1/chat/completions/?token=ignored", wantPath: "/api/v1", wantHost: "openrouter.ai"},
		{endpoint: "https://openrouter.ai:8443/api/v1/completions/", wantPath: "/api/v1", wantHost: "openrouter.ai:8443"},
		{endpoint: "https://openrouter.ai/api/v1/", wantPath: "/api/v1", wantHost: "openrouter.ai"},
	} {
		t.Run(tc.endpoint, func(t *testing.T) {
			got, err := normalizeOpenRouterEndpoint(tc.endpoint)
			if err != nil {
				t.Fatalf("normalize endpoint: %v", err)
			}
			if got.Path != tc.wantPath || got.Host != tc.wantHost || got.RawQuery != "" {
				t.Fatalf("normalized endpoint = %s, want host=%q path=%q with no query", got, tc.wantHost, tc.wantPath)
			}
		})
	}
}

func TestListTranscriptionModelsDoesNotFollowRedirects(t *testing.T) {
	requests := 0
	client := newTranscriptionModelsHTTPClient()
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://sub.openrouter.ai/api/v1/models"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	})

	_, err := listTranscriptionModels(context.Background(), provider.Config{
		Endpoint: "https://openrouter.ai/api/v1",
		APIKey:   "secret",
	}, client)
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("redirect error = %v, want HTTP 302", err)
	}
	if requests != 1 {
		t.Fatalf("redirect request count = %d, want 1", requests)
	}
}

func TestListTranscriptionModelsHonorsContextCancellation(t *testing.T) {
	started := make(chan struct{})
	client := newTranscriptionModelsHTTPClient()
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		close(started)
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := listTranscriptionModels(ctx, provider.Config{Endpoint: "https://openrouter.ai/api/v1"}, client)
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("request error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not stop after context cancellation")
	}
}

func TestListTranscriptionModelsRejectsMalformedAndOversizedResponses(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: "not json"},
		{name: "oversized", body: strings.Repeat("x", transcriptionModelsResponseLimit+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()

			_, err := listTranscriptionModels(context.Background(), provider.Config{Endpoint: "https://openrouter.ai/api/v1"}, openRouterTestClient(server))
			if err == nil {
				t.Fatal("list transcription models succeeded for invalid response")
			}
		})
	}
}

func TestListTranscriptionModelsOmitsAuthorizationWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("authorization = %q, want absent", got)
		}
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	models, err := listTranscriptionModels(context.Background(), provider.Config{Endpoint: "https://openrouter.ai/api/v1"}, openRouterTestClient(server))
	if err != nil {
		t.Fatalf("list transcription models: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("models = %#v, want empty list", models)
	}
}

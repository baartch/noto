package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"noto/internal/provider"
	"noto/internal/store"
)

func TestConfiguredSessionRequiresProvider(t *testing.T) {
	session, err := newConfiguredSession(context.Background(), nil, nil, "", nil, "", sessionCallbacks{})
	if session != nil || !errors.Is(err, errProviderSetupRequired) {
		t.Fatalf("newConfiguredSession() = (%v, %v), want nil and errProviderSetupRequired", session, err)
	}
}

func TestConfiguredSessionAllowsUnsetModelForTUISettings(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	profile := &store.Profile{ID: "profile-no-model", Slug: "profile-no-model", Name: "No Model"}
	db, err := openProfileDB(profile.Slug)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	session, err := newConfiguredSession(context.Background(), db, profile, "system prompt", &store.ProviderConfig{Endpoint: "http://127.0.0.1:1"}, "test-secret", sessionCallbacks{})
	if err != nil {
		t.Fatalf("newConfiguredSession with unset model: %v", err)
	}
	session.Close(context.Background())
}

func TestConfiguredSessionUsesProfileProviderAndPrompt(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	profile := &store.Profile{ID: "profile-test", Slug: "profile-test", Name: "Test"}
	db, err := openProfileDB(profile.Slug)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	requests := make(chan struct {
		Model         string `json:"model"`
		Authorization string
		Messages      []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}, 4)
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-secret" {
			t.Errorf("provider Authorization = %q, want test-secret", got)
		}
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode provider request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- struct {
			Model         string `json:"model"`
			Authorization string
			Messages      []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}{Model: request.Model, Authorization: r.Header.Get("Authorization"), Messages: request.Messages}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"chosen-model","choices":[{"message":{"role":"assistant","content":"hello from provider"}}]}`))
	}))
	defer providerServer.Close()

	statusUpdates := make(chan string, 8)
	statsUpdates := make(chan provider.Stats, 2)
	savingUpdates := make(chan struct{}, 2)
	callbacks := sessionCallbacks{
		OnContextStatus: func(status string) { statusUpdates <- status },
		OnStats:         func(stats provider.Stats) { statsUpdates <- stats },
		OnNotesSaving:   func() { savingUpdates <- struct{}{} },
	}
	session, err := newConfiguredSession(
		context.Background(), db, profile, "profile prompt phrase",
		&store.ProviderConfig{Endpoint: providerServer.URL, ActiveModel: "chosen-model", IsActive: true},
		"test-secret", callbacks,
	)
	if err != nil {
		t.Fatalf("newConfiguredSession: %v", err)
	}
	defer session.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := session.Send(ctx, "hello")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result.Reply != "hello from provider" {
		t.Fatalf("reply = %q", result.Reply)
	}

	select {
	case request := <-requests:
		if request.Model != "chosen-model" || request.Authorization != "Bearer test-secret" {
			t.Fatalf("provider request model/auth = %q/%q", request.Model, request.Authorization)
		}
		foundPrompt := false
		for _, message := range request.Messages {
			if message.Role == "system" && strings.Contains(message.Content, "profile prompt phrase") {
				foundPrompt = true
			}
		}
		if !foundPrompt {
			t.Fatalf("provider request omitted profile prompt: %#v", request.Messages)
		}
	case <-ctx.Done():
		t.Fatal("provider request not observed")
	}

	select {
	case status := <-statusUpdates:
		if status == "" {
			t.Fatal("context status callback received empty status")
		}
	case <-ctx.Done():
		t.Fatal("context status callback not invoked")
	}
	select {
	case <-statsUpdates:
	case <-ctx.Done():
		t.Fatal("stats callback not invoked")
	}
	select {
	case <-savingUpdates:
	case <-ctx.Done():
		t.Fatal("notes-saving callback not invoked")
	}
}

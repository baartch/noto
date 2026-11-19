package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"noto/internal/store"
)

func TestSettingsReadEndpoints(t *testing.T) {
	backend := &testSettingsBackend{}
	service := NewService(nil, &store.Profile{Name: "Artist", Slug: "artist"}, nil, "model-x", false)
	h := NewHandler(service, testOrigin, backend)

	profilesResponse := requestJSON(t, h, http.MethodGet, "/api/settings", "")
	var settings SettingsDTO
	if err := json.Unmarshal(profilesResponse, &settings); err != nil {
		t.Fatal(err)
	}
	if settings.ActiveProfileSlug != "artist" || len(settings.Profiles) != 1 || settings.Profiles[0] != (SettingsProfileDTO{Slug: "artist", Name: "Artist"}) {
		t.Fatalf("settings = %+v", settings)
	}

	modelsResponse := requestJSON(t, h, http.MethodGet, "/api/settings/models?profile=artist", "")
	var models ProfileModelsDTO
	if err := json.Unmarshal(modelsResponse, &models); err != nil {
		t.Fatal(err)
	}
	if models.ProfileSlug != "artist" || models.SavedModel != "saved-model" || len(models.Models) != 2 || models.Models[0] != "model-a" || models.Models[1] != "model-b" {
		t.Fatalf("models = %+v", models)
	}
	if recorder := request(h, http.MethodPost, "/api/settings", `{}`, testOrigin); recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/settings status = %d, want 405", recorder.Code)
	}
}

func TestSettingsModelErrorsAndRequestSecurity(t *testing.T) {
	backend := &testSettingsBackend{}
	h := NewHandler(NewService(nil, &store.Profile{Name: "Artist", Slug: "artist"}, nil, "model-x", false), testOrigin, backend)

	if recorder := request(h, http.MethodGet, "/api/settings/models", "", ""); recorder.Code != http.StatusBadRequest {
		t.Errorf("missing profile status = %d, want 400", recorder.Code)
	}
	backend.modelsErr = ErrSettingsProfileNotFound
	if recorder := request(h, http.MethodGet, "/api/settings/models?profile=missing", "", ""); recorder.Code != http.StatusNotFound {
		t.Errorf("unknown profile status = %d, want 404", recorder.Code)
	}
	backend.modelsErr = errors.New("provider returned secret credential")
	if recorder := request(h, http.MethodGet, "/api/settings/models?profile=artist", "", ""); recorder.Code != http.StatusBadGateway {
		t.Errorf("provider failure status = %d, want 502", recorder.Code)
	} else if strings.Contains(recorder.Body.String(), "secret") || strings.Contains(recorder.Body.String(), "credential") {
		t.Errorf("provider details leaked: %s", recorder.Body.String())
	}
	if recorder := requestWithHost(h, http.MethodGet, "/api/settings", "", "", "attacker.invalid"); recorder.Code != http.StatusMisdirectedRequest {
		t.Errorf("foreign host status = %d, want 421", recorder.Code)
	}
	if recorder := request(h, http.MethodGet, "/api/settings", "", "http://evil.example"); recorder.Code != http.StatusForbidden {
		t.Errorf("foreign origin status = %d, want 403", recorder.Code)
	}
}

func TestSettingsUnavailableWithoutBackend(t *testing.T) {
	h := NewHandler(NewService(nil, nil, nil, "", true), testOrigin)
	for _, path := range []string{"/api/settings", "/api/settings/models?profile=artist"} {
		if recorder := request(h, http.MethodGet, path, "", ""); recorder.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s status = %d, want 503", path, recorder.Code)
		}
	}
}

type testSettingsBackend struct {
	modelsErr error
}

func (*testSettingsBackend) Profiles(context.Context) ([]SettingsProfileDTO, error) {
	return []SettingsProfileDTO{{Slug: "artist", Name: "Artist"}}, nil
}

func (b *testSettingsBackend) Models(_ context.Context, slug string) (ProfileModelsDTO, error) {
	if b.modelsErr != nil {
		return ProfileModelsDTO{}, b.modelsErr
	}
	return ProfileModelsDTO{ProfileSlug: slug, SavedModel: "saved-model", Models: []string{"model-a", "model-b"}}, nil
}

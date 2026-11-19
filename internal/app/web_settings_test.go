package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"noto/internal/config"
	"noto/internal/profile"
	"noto/internal/security"
	"noto/internal/store"
	"noto/internal/web"
)

func TestSettingsBackendListsProfilesAndModelsWithoutMutation(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	t.Setenv("NOTO_API_KEY", "")
	t.Setenv("NOTO_ENDPOINT", "")
	t.Setenv("NOTO_MODEL", "")

	profiles := profile.NewService(nil)
	active, err := profiles.Create(context.Background(), "Alpha")
	if err != nil {
		t.Fatal(err)
	}
	configured, err := profiles.Create(context.Background(), "Beta")
	if err != nil {
		t.Fatal(err)
	}
	unconfigured, err := profiles.Create(context.Background(), "Gamma")
	if err != nil {
		t.Fatal(err)
	}
	keyless, err := profiles.Create(context.Background(), "Delta")
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WriteActiveProfile(active.Slug, time.Now()); err != nil {
		t.Fatal(err)
	}

	var failModels atomic.Bool
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Errorf("provider request = %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		if failModels.Load() {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("secret-upstream-error"))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "model-a"}, {"id": "model-b"}}})
	}))
	defer providerServer.Close()

	db, err := openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	passphrase, err := security.MachinePassphrase()
	if err != nil {
		t.Fatal(err)
	}
	credential, err := security.Encrypt("secret-key", passphrase)
	if err != nil {
		t.Fatal(err)
	}
	err = store.NewProviderConfigRepo(db).Upsert(context.Background(), &store.ProviderConfig{
		ID: "provider-beta", ProfileID: configured.ID, ProviderType: "openai_compatible",
		Endpoint: providerServer.URL + "/v1", ActiveModel: "saved-model", CredentialRef: credential, IsActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	keylessDB, err := openProfileDB(keyless.Slug)
	if err != nil {
		t.Fatal(err)
	}
	err = store.NewProviderConfigRepo(keylessDB).Upsert(context.Background(), &store.ProviderConfig{
		ID: "provider-delta", ProfileID: keyless.ID, ProviderType: "openai_compatible",
		Endpoint: providerServer.URL + "/v1", ActiveModel: "saved-without-key", IsActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := keylessDB.Close(); err != nil {
		t.Fatal(err)
	}

	backend := newWebSettingsBackend()
	choices, err := backend.Profiles(context.Background())
	if err != nil || len(choices) != 4 {
		t.Fatalf("Profiles() = %+v, err=%v", choices, err)
	}
	models, err := backend.Models(context.Background(), configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if models.ProfileSlug != configured.Slug || models.SavedModel != "saved-model" || models.SetupRequired || strings.Join(models.Models, ",") != "model-a,model-b" {
		t.Fatalf("Models() = %+v", models)
	}
	encodedModels, err := json.Marshal(models)
	if err != nil || strings.Contains(string(encodedModels), "secret-key") || strings.Contains(string(encodedModels), credential) {
		t.Fatalf("browser settings leaked credentials: %s, err=%v", encodedModels, err)
	}
	missing, err := backend.Models(context.Background(), unconfigured.Slug)
	if err != nil || !missing.SetupRequired || missing.SavedModel != "" || len(missing.Models) != 0 {
		t.Fatalf("unconfigured Models() = %+v, err=%v", missing, err)
	}
	withoutKey, err := backend.Models(context.Background(), keyless.Slug)
	if err != nil || !withoutKey.SetupRequired || withoutKey.SavedModel != "saved-without-key" || len(withoutKey.Models) != 0 {
		t.Fatalf("keyless Models() = %+v, err=%v", withoutKey, err)
	}
	if _, err := backend.Models(context.Background(), "missing-profile"); err != web.ErrSettingsProfileNotFound {
		t.Fatalf("unknown profile err = %v, want ErrSettingsProfileNotFound", err)
	}

	unchangedDB, err := openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := store.NewProviderConfigRepo(unchangedDB).GetActive(context.Background(), configured.ID)
	_ = unchangedDB.Close()
	if err != nil || stored.EffectiveModel() != "saved-model" {
		t.Fatalf("provider config after lookup = %+v, err=%v", stored, err)
	}
	activeProfile, err := profiles.GetActive(context.Background())
	if err != nil || activeProfile.Slug != active.Slug {
		t.Fatalf("active profile after lookup = %+v, err=%v", activeProfile, err)
	}

	failModels.Store(true)
	if _, err := backend.Models(context.Background(), configured.Slug); err == nil || strings.Contains(err.Error(), "secret-upstream-error") {
		t.Fatalf("upstream error = %v, want sanitized error", err)
	}
}

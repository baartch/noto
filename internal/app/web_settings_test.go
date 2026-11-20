package app

import (
	"context"
	"encoding/json"
	"errors"
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
	emptyModelProfile, err := profiles.Create(context.Background(), "Epsilon")
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WriteActiveProfile(active.Slug, time.Now()); err != nil {
		t.Fatal(err)
	}

	var failModels atomic.Bool
	var failEmbeddingModelList atomic.Bool
	var failEmbeddings atomic.Bool
	var embeddingACalls atomic.Int32
	var embeddingBCalls atomic.Int32
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.URL.Path != "/v1/models" && r.URL.Path != "/v1/embeddings/models" && r.URL.Path != "/v1/embeddings") || r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Errorf("provider request = %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		if r.URL.Path == "/v1/embeddings" {
			var request struct {
				Model string `json:"model"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			if request.Model == "embedding-b" {
				embeddingBCalls.Add(1)
			}
			if request.Model == "embedding-a" {
				embeddingACalls.Add(1)
			}
			if failEmbeddings.Load() && request.Model != "embedding-saved" {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("embedding failure"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"model": "embedding-saved", "data": []map[string]any{{"embedding": []float64{0.1, 0.2}}}})
			return
		}
		if r.URL.Path == "/v1/embeddings/models" && failEmbeddingModelList.Load() {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("secret-embedding-model-list-error"))
			return
		}
		if failModels.Load() {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("secret-upstream-error"))
			return
		}
		models := []map[string]string{{"id": "model-a"}, {"id": "model-b"}}
		if r.URL.Path == "/v1/embeddings/models" {
			models = []map[string]string{{"id": "embedding-a"}, {"id": "embedding-b"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": models})
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
		Endpoint: providerServer.URL + "/v1", ActiveModel: "saved-model", ExtractorModel: "extractor-saved",
		EmbeddingsModel: "embedding-saved", CredentialRef: credential, IsActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.NewConversationRepo(db).Create(context.Background(), &store.Conversation{ID: "conversation-beta", ProfileID: configured.ID, Status: store.ConversationArchived}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewMessageRepo(db).Create(context.Background(), &store.Message{ID: "message-beta", ConversationID: "conversation-beta", Role: store.RoleUser, Content: "beta history"}); err != nil {
		t.Fatal(err)
	}
	if err := store.NewMemoryNoteRepo(db).Create(context.Background(), &store.MemoryNote{ID: "note-beta", ProfileID: configured.ID, Category: store.CategoryFact, Content: "beta memory", Importance: 7}); err != nil {
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
	emptyModelDB, err := openProfileDB(emptyModelProfile.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.NewProviderConfigRepo(emptyModelDB).Upsert(context.Background(), &store.ProviderConfig{
		ID: "provider-epsilon", ProfileID: emptyModelProfile.ID, ProviderType: "openai_compatible",
		Endpoint: providerServer.URL + "/v1", CredentialRef: credential, IsActive: true,
	}); err != nil {
		_ = emptyModelDB.Close()
		t.Fatal(err)
	}
	if err := emptyModelDB.Close(); err != nil {
		t.Fatal(err)
	}

	backend := newWebSettingsBackend()
	choices, err := backend.Profiles(context.Background())
	if err != nil || len(choices) != 5 {
		t.Fatalf("Profiles() = %+v, err=%v", choices, err)
	}
	models, err := backend.Models(context.Background(), configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if models.ProfileSlug != configured.Slug || models.SavedModel != "saved-model" || models.SavedExtractorModel != "extractor-saved" || models.SavedEmbeddingsModel != "embedding-saved" || models.SetupRequired || strings.Join(models.Models, ",") != "model-a,model-b" || strings.Join(models.EmbeddingModels, ",") != "embedding-a,embedding-b" {
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
	failEmbeddingModelList.Store(true)
	unavailableModels, err := backend.Models(context.Background(), configured.Slug)
	if err != nil || !unavailableModels.EmbeddingModelsUnavailable || unavailableModels.SavedEmbeddingsModel != "embedding-saved" || unavailableModels.SavedExtractorModel != "extractor-saved" || strings.Join(unavailableModels.Models, ",") != "model-a,model-b" {
		t.Fatalf("settings after embeddings list failure = %+v, %v; want saved values and chat choices", unavailableModels, err)
	}
	chatOnlyDB, err := openProfileDB(active.Slug)
	if err != nil {
		t.Fatal(err)
	}
	chatOnlyRuntime := web.NewProfileRuntime(chatOnlyDB, active, nil, "", true)
	chatOnlyPreparation, err := newWebSettingsBackend().PrepareModels(context.Background(), chatOnlyRuntime, configured.Slug, web.ModelSettingsSelection{Model: "model-a"})
	if err != nil {
		_ = chatOnlyDB.Close()
		t.Fatalf("chat-only settings preparation failed when embedding list unavailable: %v", err)
	}
	chatOnlyPreparation.Abort()
	disabledEmbedding := ""
	clearEmbeddingPreparation, err := newWebSettingsBackend().PrepareModels(context.Background(), chatOnlyRuntime, configured.Slug, web.ModelSettingsSelection{Model: "model-a", EmbeddingsModel: &disabledEmbedding})
	if err != nil {
		_ = chatOnlyDB.Close()
		t.Fatalf("disabling embeddings should not require the provider model list: %v", err)
	}
	clearEmbeddingPreparation.Abort()
	changedEmbedding := "embedding-b"
	if _, err := newWebSettingsBackend().PrepareModels(context.Background(), chatOnlyRuntime, configured.Slug, web.ModelSettingsSelection{Model: "model-a", EmbeddingsModel: &changedEmbedding}); !errors.Is(err, web.ErrSettingsModelListUnavailable) {
		t.Fatalf("embedding change without model list = %v, want ErrSettingsModelListUnavailable", err)
	}
	_ = chatOnlyDB.Close()
	failEmbeddingModelList.Store(false)
	failModels.Store(true)
	chatModelsUnavailable, err := backend.Models(context.Background(), configured.Slug)
	if err != nil || !chatModelsUnavailable.ModelsUnavailable || chatModelsUnavailable.SavedModel != "saved-model" || chatModelsUnavailable.SavedExtractorModel != "extractor-saved" || chatModelsUnavailable.SavedEmbeddingsModel != "embedding-saved" || len(chatModelsUnavailable.Models) != 0 {
		t.Fatalf("settings after chat model list failure = %+v, %v; want saved values and unavailable flag", chatModelsUnavailable, err)
	}
	chatOnlyDB, err = openProfileDB(active.Slug)
	if err != nil {
		t.Fatal(err)
	}
	chatOnlyRuntime = web.NewProfileRuntime(chatOnlyDB, active, nil, "", true)
	unchangedChatPreparation, err := newWebSettingsBackend().PrepareModels(context.Background(), chatOnlyRuntime, configured.Slug, web.ModelSettingsSelection{Model: "saved-model"})
	if err != nil {
		_ = chatOnlyDB.Close()
		t.Fatalf("unchanged chat model could not be saved when list unavailable: %v", err)
	}
	unchangedChatPreparation.Abort()
	if _, err := newWebSettingsBackend().PrepareModels(context.Background(), chatOnlyRuntime, configured.Slug, web.ModelSettingsSelection{Model: "model-a"}); !errors.Is(err, web.ErrSettingsModelListUnavailable) {
		t.Fatalf("changed chat model without model list = %v, want ErrSettingsModelListUnavailable", err)
	}
	changedExtractor := "model-a"
	if _, err := newWebSettingsBackend().PrepareModels(context.Background(), chatOnlyRuntime, configured.Slug, web.ModelSettingsSelection{Model: "saved-model", ExtractorModel: &changedExtractor}); !errors.Is(err, web.ErrSettingsModelListUnavailable) {
		t.Fatalf("changed extractor model without model list = %v, want ErrSettingsModelListUnavailable", err)
	}
	if _, err := newWebSettingsBackend().PrepareModels(context.Background(), chatOnlyRuntime, emptyModelProfile.Slug, web.ModelSettingsSelection{}); !errors.Is(err, web.ErrSettingsModelNotFound) {
		t.Fatalf("empty saved chat model with unavailable list = %v, want ErrSettingsModelNotFound", err)
	}
	_ = chatOnlyDB.Close()
	failModels.Store(false)

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

	currentDB, err := openProfileDB(active.Slug)
	if err != nil {
		t.Fatal(err)
	}
	service := web.NewService(currentDB, active, nil, "", true)
	openFailureBackend := newWebSettingsBackend()
	openFailureBackend.openDB = func(string) (*store.DB, error) { return nil, errors.New("open failure") }
	if _, err := service.SaveSettings(context.Background(), openFailureBackend, configured.Slug, "model-a", 1); err == nil {
		t.Fatal("SaveSettings succeeded after destination DB open failure")
	}
	if state := service.State(); state.Profile.Slug != active.Slug || state.Revision != 1 {
		t.Fatalf("runtime changed after DB open failure: %+v", state)
	}
	invalidModelBackend := newWebSettingsBackend()
	if _, err := service.SaveSettings(context.Background(), invalidModelBackend, configured.Slug, "unknown-model", 1); !errors.Is(err, web.ErrSettingsModelNotFound) {
		t.Fatalf("unknown model error = %v, want ErrSettingsModelNotFound", err)
	}
	invalidEmbeddingBackend := newWebSettingsBackend()
	unknownEmbedding := "unknown-embedding"
	if _, err := service.SaveModelSettings(context.Background(), invalidEmbeddingBackend, configured.Slug, web.ModelSettingsSelection{Model: "model-a", EmbeddingsModel: &unknownEmbedding}, 1); !errors.Is(err, web.ErrSettingsModelNotFound) {
		t.Fatalf("unknown embeddings model error = %v, want ErrSettingsModelNotFound", err)
	}
	invalidExtractorBackend := newWebSettingsBackend()
	unknownExtractor := "embedding-a"
	if _, err := service.SaveModelSettings(context.Background(), invalidExtractorBackend, configured.Slug, web.ModelSettingsSelection{Model: "model-a", ExtractorModel: &unknownExtractor}, 1); !errors.Is(err, web.ErrSettingsModelNotFound) {
		t.Fatalf("embedding-only extractor model error = %v, want ErrSettingsModelNotFound", err)
	}
	startupFailureBackend := newWebSettingsBackend()
	startupFailureBackend.newSession = func(context.Context, *store.DB, *store.Profile, string, *store.ProviderConfig, string) (web.ChatSession, error) {
		return nil, errors.New("session startup failed")
	}
	if _, err := service.SaveSettings(context.Background(), startupFailureBackend, configured.Slug, "model-a", 1); err == nil {
		t.Fatal("SaveSettings succeeded after session startup failure")
	}
	if state := service.State(); state.Profile.Slug != active.Slug || state.Revision != 1 {
		t.Fatalf("runtime changed after session startup failure: %+v", state)
	}
	failedSwitchBackend := newWebSettingsBackend()
	failedSwitchBackend.writeActive = func(string, time.Time) error { return errors.New("marker write failed") }
	newEmbeddingModel := "embedding-b"
	if _, err := service.SaveModelSettings(context.Background(), failedSwitchBackend, configured.Slug, web.ModelSettingsSelection{
		Model: "model-a", EmbeddingsModel: &newEmbeddingModel,
	}, 1); err == nil {
		t.Fatal("profile switch succeeded after active marker write failure")
	}
	if got := embeddingBCalls.Load(); got != 0 {
		t.Fatalf("failed profile switch re-embedded %d notes before profile marker commit", got)
	}
	if state := service.State(); state.Profile.Slug != active.Slug || state.Revision != 1 {
		t.Fatalf("runtime changed after embedding profile switch failure: %+v", state)
	}
	failEmbeddings.Store(true)
	if _, err := service.SaveModelSettings(context.Background(), newWebSettingsBackend(), configured.Slug, web.ModelSettingsSelection{
		Model: "model-a", EmbeddingsModel: &newEmbeddingModel,
	}, 1); err == nil {
		t.Fatal("profile switch succeeded after candidate embedding rebuild failed")
	}
	failEmbeddings.Store(false)
	if state := service.State(); state.Profile.Slug != active.Slug || state.Revision != 1 {
		t.Fatalf("runtime changed after candidate embedding failure: %+v", state)
	}
	activeAfterCandidateFailure, err := profiles.GetActive(context.Background())
	if err != nil || activeAfterCandidateFailure.Slug != active.Slug {
		t.Fatalf("active profile after candidate embedding failure = %+v, err=%v", activeAfterCandidateFailure, err)
	}
	candidateDB, err := openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	candidateConfig, err := store.NewProviderConfigRepo(candidateDB).GetActive(context.Background(), configured.ID)
	_ = candidateDB.Close()
	if err != nil || candidateConfig.EmbeddingsModel != "embedding-saved" {
		t.Fatalf("destination config after candidate failure = %+v, err=%v", candidateConfig, err)
	}
	commitFailureBackend := newWebSettingsBackend()
	commitFailureBackend.persistModel = func(context.Context, *store.DB, string, string) error {
		return errors.New("injected transaction commit failure")
	}
	if _, err := service.SaveSettings(context.Background(), commitFailureBackend, configured.Slug, "model-a", 1); err == nil {
		t.Fatal("SaveSettings succeeded after model transaction commit failure")
	}
	if state := service.State(); state.Profile.Slug != active.Slug || state.Revision != 1 {
		t.Fatalf("runtime changed after model transaction commit failure: %+v", state)
	}
	activeAfterStartupFailure, err := profiles.GetActive(context.Background())
	if err != nil || activeAfterStartupFailure.Slug != active.Slug {
		t.Fatalf("active profile after session startup failure = %+v, err=%v", activeAfterStartupFailure, err)
	}
	startupDB, err := openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	startupConfig, err := store.NewProviderConfigRepo(startupDB).GetActive(context.Background(), configured.ID)
	_ = startupDB.Close()
	if err != nil || startupConfig.EffectiveModel() != "saved-model" {
		t.Fatalf("provider config after session startup failure = %+v, err=%v", startupConfig, err)
	}
	failureDB, err := openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failureDB.ExecContext(context.Background(), `CREATE TRIGGER fail_model_update BEFORE UPDATE OF active_model ON provider_config BEGIN SELECT RAISE(ABORT, 'injected model write failure'); END`); err != nil {
		_ = failureDB.Close()
		t.Fatal(err)
	}
	_ = failureDB.Close()
	modelFailureBackend := newWebSettingsBackend()
	if _, err := service.SaveSettings(context.Background(), modelFailureBackend, configured.Slug, "model-a", 1); err == nil {
		t.Fatal("SaveSettings succeeded after model persistence failure")
	}
	if state := service.State(); state.Profile.Slug != active.Slug || state.Revision != 1 {
		t.Fatalf("runtime changed after model persistence failure: %+v", state)
	}
	triggerDB, err := openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := triggerDB.ExecContext(context.Background(), `DROP TRIGGER fail_model_update`); err != nil {
		_ = triggerDB.Close()
		t.Fatal(err)
	}
	_ = triggerDB.Close()

	createdSessions := 0
	createSession := backend.newSession
	backend.newSession = func(ctx context.Context, db *store.DB, p *store.Profile, prompt string, cfg *store.ProviderConfig, key string) (web.ChatSession, error) {
		createdSessions++
		return createSession(ctx, db, p, prompt, cfg, key)
	}
	embeddingA, extractorFallback := "embedding-a", ""
	saved, err := service.SaveModelSettings(context.Background(), backend, configured.Slug, web.ModelSettingsSelection{
		Model: "model-a", EmbeddingsModel: &embeddingA, ExtractorModel: &extractorFallback,
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Profile.Slug != configured.Slug || saved.Model != "model-a" || saved.SetupRequired || saved.Revision != 2 {
		t.Fatalf("saved state = %+v", saved)
	}
	if got := embeddingACalls.Load(); got == 0 {
		t.Fatal("profile switch did not rebuild the destination profile's embedding index")
	}
	history, err := service.Messages(context.Background(), nil, 50)
	if err != nil || len(history.Items) != 1 || history.Items[0].Content != "beta history" {
		t.Fatalf("profile B history = %+v, err=%v", history, err)
	}
	memory, err := service.Memory(context.Background(), "daily", 0, 50)
	if err != nil || len(memory.Items) != 1 || memory.Items[0].Content != "beta memory" {
		t.Fatalf("profile B memory = %+v, err=%v", memory, err)
	}
	if createdSessions != 1 {
		t.Fatalf("session creations after profile switch = %d, want 1", createdSessions)
	}
	updated, err := service.SaveSettings(context.Background(), backend, configured.Slug, "model-b", saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Profile.Slug != configured.Slug || updated.Model != "model-b" || updated.Revision != 3 || createdSessions != 1 {
		t.Fatalf("same-profile save = %+v; sessions created=%d", updated, createdSessions)
	}
	failEmbeddingModelList.Store(true)
	disabledEmbedding, selectedExtractor := "", "model-a"
	withoutEmbeddingList, err := service.SaveModelSettings(context.Background(), backend, configured.Slug, web.ModelSettingsSelection{
		Model: "model-b", EmbeddingsModel: &disabledEmbedding, ExtractorModel: &selectedExtractor,
	}, updated.Revision)
	if err != nil || withoutEmbeddingList.Revision != 4 {
		t.Fatalf("clear embeddings/change extractor while list unavailable = %+v, %v", withoutEmbeddingList, err)
	}
	failEmbeddingModelList.Store(false)
	embeddingsModel, extractorModel := "embedding-b", ""
	withAuxiliaryModels, err := service.SaveModelSettings(context.Background(), backend, configured.Slug, web.ModelSettingsSelection{
		Model: "model-b", EmbeddingsModel: &embeddingsModel, ExtractorModel: &extractorModel,
	}, withoutEmbeddingList.Revision)
	if err != nil || withAuxiliaryModels.Model != "model-b" || withAuxiliaryModels.Revision != 5 {
		t.Fatalf("save embeddings/extractor models = %+v, %v", withAuxiliaryModels, err)
	}
	failEmbeddings.Store(true)
	failingEmbeddings := "embedding-a"
	if _, err := service.SaveModelSettings(context.Background(), backend, configured.Slug, web.ModelSettingsSelection{
		Model: "model-a", EmbeddingsModel: &failingEmbeddings, ExtractorModel: &extractorModel,
	}, withAuxiliaryModels.Revision); err == nil {
		t.Fatal("settings save succeeded after embeddings provider failure")
	}
	failEmbeddings.Store(false)
	if state := service.State(); state.Model != "model-b" || state.Revision != withAuxiliaryModels.Revision {
		t.Fatalf("runtime changed after failed embeddings rebuild: %+v", state)
	}
	omittedModels, err := service.SaveModelSettings(context.Background(), backend, configured.Slug, web.ModelSettingsSelection{Model: "model-b"}, withAuxiliaryModels.Revision)
	if err != nil || omittedModels.Revision != 6 {
		t.Fatalf("save with omitted auxiliary models = %+v, %v", omittedModels, err)
	}
	verifyAuxDB, err := openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	auxConfig, err := store.NewProviderConfigRepo(verifyAuxDB).GetActive(context.Background(), configured.ID)
	_ = verifyAuxDB.Close()
	if err != nil || auxConfig.EmbeddingsModel != "embedding-b" || auxConfig.ExtractorModel != "" {
		t.Fatalf("auxiliary config after failure/omitted fields = %+v, err=%v", auxConfig, err)
	}
	if err := service.CloseRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if currentActive, err := profiles.GetActive(context.Background()); err != nil || currentActive.Slug != configured.Slug {
		t.Fatalf("active profile after save = %+v, err=%v", currentActive, err)
	}
	verifyDB, err := openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	updatedConfig, err := store.NewProviderConfigRepo(verifyDB).GetActive(context.Background(), configured.ID)
	_ = verifyDB.Close()
	if err != nil || updatedConfig.EffectiveModel() != "model-b" || updatedConfig.EmbeddingsModel != "embedding-b" || updatedConfig.ExtractorModel != "" {
		t.Fatalf("provider config after save = %+v, err=%v", updatedConfig, err)
	}
	if err := config.WriteActiveProfile(active.Slug, time.Now()); err != nil {
		t.Fatal(err)
	}
	rollbackDB, err := openProfileDB(active.Slug)
	if err != nil {
		t.Fatal(err)
	}
	rollbackService := web.NewService(rollbackDB, active, nil, "", true)
	failingBackend := newWebSettingsBackend()
	failingBackend.writeActive = func(string, time.Time) error { return errors.New("marker write failed") }
	if _, err := rollbackService.SaveSettings(context.Background(), failingBackend, configured.Slug, "model-a", 1); err == nil {
		t.Fatal("SaveSettings succeeded after active marker write failed")
	}
	if state := rollbackService.State(); state.Profile.Slug != active.Slug || state.Revision != 1 {
		t.Fatalf("runtime changed after failed settings save: %+v", state)
	}
	if err := rollbackService.CloseRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	activeProfile, err = profiles.GetActive(context.Background())
	if err != nil || activeProfile.Slug != active.Slug {
		t.Fatalf("active profile after failed save = %+v, err=%v", activeProfile, err)
	}
	verifyDB, err = openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	rolledBackConfig, err := store.NewProviderConfigRepo(verifyDB).GetActive(context.Background(), configured.ID)
	_ = verifyDB.Close()
	if err != nil || rolledBackConfig.EffectiveModel() != "model-b" {
		t.Fatalf("provider config after failed save = %+v, err=%v", rolledBackConfig, err)
	}
	if err := config.WriteActiveProfile(active.Slug, time.Now()); err != nil {
		t.Fatal(err)
	}
	recoveryDB, err := openProfileDB(configured.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recoveryDB.ExecContext(context.Background(), `CREATE TRIGGER fail_model_rollback BEFORE UPDATE OF active_model ON provider_config WHEN NEW.active_model = 'model-b' BEGIN SELECT RAISE(ABORT, 'injected rollback failure'); END`); err != nil {
		_ = recoveryDB.Close()
		t.Fatal(err)
	}
	_ = recoveryDB.Close()
	recoveryCurrentDB, err := openProfileDB(active.Slug)
	if err != nil {
		t.Fatal(err)
	}
	recoveryService := web.NewService(recoveryCurrentDB, active, nil, "", true)
	recoveryBackend := newWebSettingsBackend()
	recoveryBackend.writeActive = func(string, time.Time) error { return errors.New("marker write failed") }
	if _, err := recoveryService.SaveSettings(context.Background(), recoveryBackend, configured.Slug, "model-a", 1); !errors.Is(err, web.ErrSettingsRecoveryRequired) {
		t.Fatalf("SaveSettings error after rollback failure = %v, want ErrSettingsRecoveryRequired", err)
	}
	if _, err := recoveryService.SaveSettings(context.Background(), recoveryBackend, configured.Slug, "model-b", 1); !errors.Is(err, web.ErrSettingsRecoveryRequired) {
		t.Fatalf("backend error after recovery required = %v, want ErrSettingsRecoveryRequired", err)
	}
	if state := recoveryService.State(); state.Profile.Slug != active.Slug || state.Revision != 1 || !state.RecoveryRequired {
		t.Fatalf("runtime changed after rollback failure: %+v", state)
	}
	if err := recoveryService.CloseRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}

	failModels.Store(true)
	chatModels, err := backend.Models(context.Background(), configured.Slug)
	if err != nil || !chatModels.ModelsUnavailable || chatModels.SavedModel == "" || chatModels.SavedEmbeddingsModel != "embedding-b" || chatModels.SavedExtractorModel != "" {
		t.Fatalf("settings after provider list failure = %+v, %v; want saved values", chatModels, err)
	}
}

func TestSettingsSaveSelectsUnconfiguredProfile(t *testing.T) {
	t.Setenv("NOTO_APP_DIR", t.TempDir())
	t.Setenv("NOTO_API_KEY", "")
	t.Setenv("NOTO_ENDPOINT", "")
	t.Setenv("NOTO_MODEL", "")
	profiles := profile.NewService(nil)
	alpha, err := profiles.Create(context.Background(), "Alpha")
	if err != nil {
		t.Fatal(err)
	}
	beta, err := profiles.Create(context.Background(), "Beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WriteActiveProfile(alpha.Slug, time.Now()); err != nil {
		t.Fatal(err)
	}
	db, err := openProfileDB(alpha.Slug)
	if err != nil {
		t.Fatal(err)
	}
	service := web.NewService(db, alpha, nil, "", true)
	backend := newWebSettingsBackend()
	if _, err := service.SaveSettings(context.Background(), backend, "missing-profile", "", 1); !errors.Is(err, web.ErrSettingsProfileNotFound) {
		t.Fatalf("unknown profile error = %v, want ErrSettingsProfileNotFound", err)
	}
	if _, err := service.SaveSettings(context.Background(), backend, beta.Slug, "not-a-model", 1); !errors.Is(err, web.ErrSettingsModelNotFound) {
		t.Fatalf("unconfigured model error = %v, want ErrSettingsModelNotFound", err)
	}
	state, err := service.SaveSettings(context.Background(), backend, beta.Slug, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if state.Profile.Slug != beta.Slug || state.Model != "" || !state.SetupRequired || state.Revision != 2 {
		t.Fatalf("unconfigured profile state = %+v", state)
	}
	if err := service.CloseRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	active, err := profiles.GetActive(context.Background())
	if err != nil || active.Slug != beta.Slug {
		t.Fatalf("active profile = %+v, err=%v", active, err)
	}
}

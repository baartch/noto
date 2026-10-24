package commands

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"noto/internal/config"
	"noto/internal/provider"
	"noto/internal/store"
)

const (
	extractorTestSlug      = "test-profile"
	extractorTestProfileID = "profile-1"
)

// newExtractorTestEnv points NOTO_APP_DIR at a temp dir, seeds a profile DB
// with an active provider_config row, and returns an ExecContext wired with
// the given ListModels hook (nil means no hook).
func newExtractorTestEnv(t *testing.T, hook func(context.Context) ([]provider.ModelInfo, error)) *ExecContext {
	t.Helper()
	t.Setenv("NOTO_APP_DIR", t.TempDir())

	if err := config.EnsureAppDirs(extractorTestSlug); err != nil {
		t.Fatalf("ensure app dirs: %v", err)
	}
	dbPath, err := config.ProfileDBPath(extractorTestSlug)
	if err != nil {
		t.Fatalf("profile db path: %v", err)
	}
	seedDB, err := store.OpenProfile(dbPath)
	if err != nil {
		t.Fatalf("open profile db: %v", err)
	}
	defer func() {
		_ = seedDB.Close()
	}()
	repo := store.NewProviderConfigRepo(seedDB)
	if err := repo.Upsert(context.Background(), &store.ProviderConfig{
		ID:           "pc-1",
		ProfileID:    extractorTestProfileID,
		ProviderType: "openai_compatible",
		Endpoint:     "https://api.example.com/v1",
		Model:        "default-model",
		IsActive:     true,
	}); err != nil {
		t.Fatalf("seed provider config: %v", err)
	}

	return &ExecContext{
		ProfileID:   extractorTestProfileID,
		ProfileSlug: extractorTestSlug,
		Output:      &bytes.Buffer{},
		ListModels:  hook,
	}
}

// persistedExtractorModel reads the extractor_model stored for the test profile.
func persistedExtractorModel(t *testing.T) string {
	t.Helper()
	dbPath, err := config.ProfileDBPath(extractorTestSlug)
	if err != nil {
		t.Fatalf("profile db path: %v", err)
	}
	db, err := store.OpenProfile(dbPath)
	if err != nil {
		t.Fatalf("open profile db: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()
	cfg, err := store.NewProviderConfigRepo(db).GetActive(context.Background(), extractorTestProfileID)
	if err != nil {
		t.Fatalf("get active provider config: %v", err)
	}
	return cfg.ExtractorModel
}

func TestModelExtractorHandler_RejectsIneligibleModels(t *testing.T) {
	models := []provider.ModelInfo{
		{ID: "good", SupportsStructuredOutputs: true},
		{ID: "bad"},
	}

	t.Run("eligible model is persisted", func(t *testing.T) {
		ctx := newExtractorTestEnv(t, func(context.Context) ([]provider.ModelInfo, error) {
			return models, nil
		})
		if err := modelExtractorHandler(ctx, []string{"good"}); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got := persistedExtractorModel(t); got != "good" {
			t.Fatalf("expected extractor_model %q, got %q", "good", got)
		}
		if out := ctx.Output.(*bytes.Buffer).String(); !strings.Contains(out, "Extractor model set to: good") {
			t.Fatalf("unexpected output: %q", out)
		}
	})

	t.Run("ineligible model is refused", func(t *testing.T) {
		ctx := newExtractorTestEnv(t, func(context.Context) ([]provider.ModelInfo, error) {
			return models, nil
		})
		err := modelExtractorHandler(ctx, []string{"bad"})
		if err == nil || !strings.Contains(err.Error(), "does not support structured outputs") {
			t.Fatalf("expected structured-outputs refusal, got %v", err)
		}
	})

	t.Run("missing model is refused", func(t *testing.T) {
		ctx := newExtractorTestEnv(t, func(context.Context) ([]provider.ModelInfo, error) {
			return models, nil
		})
		err := modelExtractorHandler(ctx, []string{"missing"})
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected not-found refusal, got %v", err)
		}
	})

	t.Run("model list fetch error is refused", func(t *testing.T) {
		ctx := newExtractorTestEnv(t, func(context.Context) ([]provider.ModelInfo, error) {
			return nil, errors.New("network down")
		})
		err := modelExtractorHandler(ctx, []string{"good"})
		if err == nil || !strings.Contains(err.Error(), "could not verify") {
			t.Fatalf("expected could-not-verify refusal, got %v", err)
		}
	})

	t.Run("nil model list hook is refused", func(t *testing.T) {
		ctx := newExtractorTestEnv(t, nil)
		err := modelExtractorHandler(ctx, []string{"good"})
		if err == nil || !strings.Contains(err.Error(), "model list unavailable") {
			t.Fatalf("expected model-list-unavailable refusal, got %v", err)
		}
	})

	t.Run("no args opens the picker", func(t *testing.T) {
		ctx := newExtractorTestEnv(t, func(context.Context) ([]provider.ModelInfo, error) {
			return models, nil
		})
		err := modelExtractorHandler(ctx, nil)
		if !AsErrOpenExtractorModelPicker(err) {
			t.Fatalf("expected ErrOpenExtractorModelPicker, got %v", err)
		}
	})

	t.Run("refused models are not persisted", func(t *testing.T) {
		ctx := newExtractorTestEnv(t, func(context.Context) ([]provider.ModelInfo, error) {
			return models, nil
		})
		if err := modelExtractorHandler(ctx, []string{"bad"}); err == nil {
			t.Fatal("expected refusal for ineligible model")
		}
		if err := modelExtractorHandler(ctx, []string{"missing"}); err == nil {
			t.Fatal("expected refusal for missing model")
		}
		if got := persistedExtractorModel(t); got != "" {
			t.Fatalf("expected extractor_model to remain empty, got %q", got)
		}
	})
}

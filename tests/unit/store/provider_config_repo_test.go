package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"noto/internal/store"
)

func TestProviderConfigRepo_SetCredentialRef_CreatesConfigWhenMissing(t *testing.T) {
	db := openUnitTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repo := store.NewProviderConfigRepo(db)
	profileID := "profile-1"

	if err := repo.SetCredentialRef(context.Background(), profileID, "enc-key"); err != nil {
		t.Fatalf("set credential ref: %v", err)
	}

	cfg, err := repo.GetActive(context.Background(), profileID)
	if err != nil {
		t.Fatalf("get active config: %v", err)
	}
	if cfg.CredentialRef != "enc-key" {
		t.Fatalf("credential ref mismatch: got %q", cfg.CredentialRef)
	}
	if cfg.ProviderType != "openai_compatible" {
		t.Fatalf("provider type mismatch: got %q", cfg.ProviderType)
	}
}

func TestProviderConfigRepo_SetEndpoint_CreatesConfigWhenMissing(t *testing.T) {
	db := openUnitTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repo := store.NewProviderConfigRepo(db)
	profileID := "profile-2"

	if err := repo.SetEndpoint(context.Background(), profileID, "https://api.example.com/v1"); err != nil {
		t.Fatalf("set endpoint: %v", err)
	}

	cfg, err := repo.GetActive(context.Background(), profileID)
	if err != nil {
		t.Fatalf("get active config: %v", err)
	}
	if cfg.Endpoint != "https://api.example.com/v1" {
		t.Fatalf("endpoint mismatch: got %q", cfg.Endpoint)
	}
	if cfg.ProviderType != "openai_compatible" {
		t.Fatalf("provider type mismatch: got %q", cfg.ProviderType)
	}
}

func openUnitTestDB(t *testing.T) *store.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "memory.db")
	db, err := store.OpenForTesting(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return db
}

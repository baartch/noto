package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"noto/internal/profile"
	"noto/internal/store"
)

func TestProfileMetadata_CreateWritesFile(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()

	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))
	p, err := svc.Create(ctx, "Metadata Profile")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	path, err := profile.MetadataPath(p.Slug)
	if err != nil {
		t.Fatalf("MetadataPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected metadata file to be non-empty")
	}

	meta, err := profile.ReadMetadata(p.Slug)
	if err != nil {
		t.Fatalf("ReadMetadata: %v", err)
	}
	if meta.Name != p.Name {
		t.Errorf("expected name=%s, got %s", p.Name, meta.Name)
	}
	if meta.Slug != p.Slug {
		t.Errorf("expected slug=%s, got %s", p.Slug, meta.Slug)
	}
}

func TestProfileMetadata_PersistedInProfileDir(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()

	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))
	p, err := svc.Create(ctx, "Metadata Dir")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	path, err := profile.MetadataPath(p.Slug)
	if err != nil {
		t.Fatalf("MetadataPath: %v", err)
	}

	profileDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if _, err := os.Stat(profileDir); err != nil {
		t.Fatalf("expected profile dir to exist: %v", err)
	}
	if filepath.Base(profileDir) != p.Slug {
		t.Errorf("expected profile dir base=%s, got %s", p.Slug, filepath.Base(profileDir))
	}
}

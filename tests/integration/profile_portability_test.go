package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"noto/internal/config"
	"noto/internal/profile"
	"noto/internal/store"
)

func TestProfilePortability_MoveProfileDirectory(t *testing.T) {
	// Instance A
	dbA, closeA := tempDB(t)
	defer closeA()
	ctx := context.Background()
	svcA := profile.NewService(store.NewProfileRepo(dbA))
	p, err := svcA.Create(ctx, "Portable")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	profileDir, err := config.ProfileDir(p.Slug)
	if err != nil {
		t.Fatalf("profile dir: %v", err)
	}
	metaPath, err := profile.MetadataPath(p.Slug)
	if err != nil {
		t.Fatalf("metadata path: %v", err)
	}
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("expected metadata file: %v", err)
	}

	// Move to instance B
	instanceB := t.TempDir()
	t.Setenv("NOTO_APP_DIR", instanceB)
	profilesDir, err := config.ProfilesDir()
	if err != nil {
		t.Fatalf("profiles dir: %v", err)
	}
	if err := os.MkdirAll(profilesDir, 0o700); err != nil {
		t.Fatalf("mkdir profiles dir: %v", err)
	}
	destDir := filepath.Join(profilesDir, p.Slug)
	if err := os.Rename(profileDir, destDir); err != nil {
		t.Fatalf("move profile dir: %v", err)
	}

	profiles, warnings, err := profile.DiscoverProfiles()
	if err != nil {
		t.Fatalf("discover profiles: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].Name != p.Name {
		t.Errorf("expected name=%s, got %s", p.Name, profiles[0].Name)
	}
	if profiles[0].Slug != p.Slug {
		t.Errorf("expected slug=%s, got %s", p.Slug, profiles[0].Slug)
	}
}

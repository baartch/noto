package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"noto/internal/profile"
)

func TestGlobalDB_NotCreatedByProfileOps(t *testing.T) {
	ctx := context.Background()
	appDir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", appDir)

	profilesDir := filepath.Join(appDir, "profiles")
	if err := os.MkdirAll(profilesDir, 0o700); err != nil {
		t.Fatalf("mkdir profiles dir: %v", err)
	}

	svc := profile.NewService(nil)
	if _, err := svc.Create(ctx, "Global Exclusion"); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	globalPath := filepath.Join(appDir, "global.db")
	if _, err := os.Stat(globalPath); !os.IsNotExist(err) {
		t.Fatalf("expected global.db to be absent, got err=%v", err)
	}
}

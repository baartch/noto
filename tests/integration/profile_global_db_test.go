package integration

import (
	"context"
	"path/filepath"
	"testing"

	"noto/internal/profile"
	"noto/internal/store"
)

func TestGlobalDB_NoProfileTable(t *testing.T) {
	ctx := context.Background()
	appDir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", appDir)

	globalPath := filepath.Join(appDir, "global.db")
	db, err := store.OpenGlobal(globalPath)
	if err != nil {
		t.Fatalf("open global db: %v", err)
	}
	svc := profile.NewService(store.NewProfileRepo(db))
	if _, err := svc.Create(ctx, "Global Exclusion"); err != nil {
		_ = db.Close()
		t.Fatalf("create profile: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	db2, err := store.OpenGlobal(globalPath)
	if err != nil {
		t.Fatalf("open global db again: %v", err)
	}
	defer func() { _ = db2.Close() }()

	var count int
	if err := db2.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'profiles'`,
	).Scan(&count); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no profiles table, found %d", count)
	}
}

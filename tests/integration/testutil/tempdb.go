package testutil

import (
	"path/filepath"
	"testing"

	"noto/internal/store"
)

// TempDB creates a temporary SQLite database and returns the DB handle and a closer.
func TempDB(t *testing.T) (*store.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", dir)
	path := filepath.Join(dir, "test.db")
	db, err := store.OpenForTesting(path)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	return db, func() { _ = db.Close() }
}

package integration

import (
	"path/filepath"
	"testing"

	"noto/internal/store"
)

func tempDBForBenchmark(tb testing.TB) (*store.DB, func()) {
	tb.Helper()
	dir := tb.TempDir()
	tb.Setenv("NOTO_APP_DIR", dir)
	path := filepath.Join(dir, "test.db")
	db, err := store.OpenForTesting(path)
	if err != nil {
		tb.Fatalf("open temp db: %v", err)
	}
	return db, func() { _ = db.Close() }
}

package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContextCacheMigrationPreservesProfileData(t *testing.T) {
	for _, missingCache := range []bool{true, false} {
		name := "existing cache"
		if missingCache {
			name = "missing cache"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "memory.db")
			legacy, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := legacy.ExecContext(ctx, `CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT (datetime('now')))`); err != nil {
				t.Fatal(err)
			}
			for _, migration := range []string{"0001_init.sql", "0002_extractor_model.sql", "0002_system_prompts.sql", "0003_memory_timeline.sql", "0004_reference_files.sql"} {
				contents, err := profileMigrationsFS.ReadFile("migrations/profile/" + migration)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := legacy.ExecContext(ctx, string(contents)); err != nil {
					t.Fatalf("apply legacy migration %s: %v", migration, err)
				}
				if _, err := legacy.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, strings.TrimSuffix(migration, ".sql")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := legacy.ExecContext(ctx, `INSERT INTO memory_notes (id, profile_id, content) VALUES ('note-1', 'profile-1', 'existing note')`); err != nil {
				t.Fatal(err)
			}
			if missingCache {
				if _, err := legacy.ExecContext(ctx, `DROP TABLE context_cache`); err != nil {
					t.Fatal(err)
				}
			} else if _, err := legacy.ExecContext(ctx, `INSERT INTO context_cache (id, profile_id, cache_key, payload) VALUES ('cache-1', 'profile-1', 'key', 'existing payload')`); err != nil {
				t.Fatal(err)
			}
			if err := legacy.Close(); err != nil {
				t.Fatal(err)
			}

			db, err := OpenProfile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var content string
			if err := db.QueryRowContext(ctx, `SELECT content FROM memory_notes WHERE id = 'note-1'`).Scan(&content); err != nil || content != "existing note" {
				t.Fatalf("existing note: content = %q, err = %v", content, err)
			}
			repo := NewContextCacheRepo(db)
			if !missingCache {
				entry, err := repo.Get(ctx, "profile-1", "key")
				if err != nil || entry.Payload != "existing payload" {
					t.Fatalf("existing cache entry: entry = %#v, err = %v", entry, err)
				}
			}
			if err := repo.Upsert(ctx, &ContextCacheEntry{ID: "cache-2", ProfileID: "profile-1", CacheKey: "new-key", Payload: "new payload", CreatedAt: time.Now().UTC()}); err != nil {
				t.Fatalf("write cache after migration: %v", err)
			}
			entry, err := repo.Get(ctx, "profile-1", "new-key")
			if err != nil || entry.Payload != "new payload" {
				t.Fatalf("new cache entry: entry = %#v, err = %v", entry, err)
			}
			if err := repo.InvalidateAll(ctx, "profile-1"); err != nil {
				t.Fatalf("invalidate cache after migration: %v", err)
			}
			var indexCount int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_context_cache_profile_id'`).Scan(&indexCount); err != nil || indexCount != 1 {
				t.Fatalf("cache index count = %d, err = %v", indexCount, err)
			}
		})
	}
}

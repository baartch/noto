package store

import (
	"context"
	"path/filepath"
	"testing"

	"noto/internal/vector"
)

func TestSetManifestEmbeddingModel(t *testing.T) {
	db, err := OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewVectorManifestRepo(db)
	ctx := context.Background()
	if err := repo.UpsertManifest(ctx, &VectorManifest{
		ID: "manifest", ProfileID: "profile", IndexPath: "memory.vec",
		IndexFormatVersion: "1", EmbeddingModel: "old-model", Status: VectorManifestReady,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetManifestEmbeddingModel(ctx, "profile", "new-model", 3); err != nil {
		t.Fatalf("SetManifestEmbeddingModel: %v", err)
	}
	manifest, err := repo.GetManifest(ctx, "profile")
	if err != nil || manifest.EmbeddingModel != "new-model" || manifest.EmbeddingDim != 3 {
		t.Fatalf("manifest = %#v, %v; want model new-model dim 3", manifest, err)
	}
}

func TestReplaceEntriesAndEmbeddingUpdatesModelDimensionAndRemovesStaleRows(t *testing.T) {
	db, err := OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewVectorManifestRepo(db)
	ctx := context.Background()
	if err := repo.UpsertManifest(ctx, &VectorManifest{
		ID: "manifest", ProfileID: "profile", IndexPath: "memory.vec",
		IndexFormatVersion: "1", EmbeddingModel: "old-model", EmbeddingDim: 2,
		Status: VectorManifestReady,
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"stale", "current"} {
		if err := repo.UpsertEntry(ctx, &vector.ManifestEntry{
			ID: "ve-" + id, ProfileID: "profile", SourceType: vector.SourceMemoryNote,
			SourceID: id, ChunkHash: id, EmbeddingModel: "old-model", EmbeddingDim: 2, VectorRef: "0",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.ReplaceEntriesAndEmbedding(ctx, "profile", "new-model", 3, []*vector.ManifestEntry{{
		ID: "ve-current", ProfileID: "profile", SourceType: vector.SourceMemoryNote,
		SourceID: "current", ChunkHash: "current", EmbeddingModel: "new-model",
		EmbeddingDim: 3, VectorRef: "0",
	}}); err != nil {
		t.Fatalf("ReplaceEntriesAndEmbedding: %v", err)
	}
	manifest, err := repo.GetManifest(ctx, "profile")
	if err != nil || manifest.EmbeddingModel != "new-model" || manifest.EmbeddingDim != 3 {
		t.Fatalf("manifest = %#v, %v; want model new-model dim 3", manifest, err)
	}
	entries, err := repo.ListEntriesVec(ctx, "profile")
	if err != nil || len(entries) != 1 || entries[0].SourceID != "current" {
		t.Fatalf("entries = %#v, %v; want only current", entries, err)
	}
}

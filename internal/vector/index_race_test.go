package vector

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	vecfile "noto/internal/vector/file"
	"noto/internal/vector/hnsw"
)

// TestFileIndexConcurrentAccessIsSafe pins that FileIndex tolerates the
// concurrent access noto creates: chat-turn saves (dedup Search + Upsert) can
// overlap background extraction (Search/Flush) on the same index. Run with
// -race; a data race here is a fatal crash in production.
func TestFileIndexConcurrentAccessIsSafe(t *testing.T) {
	idx := NewFileIndex(filepath.Join(t.TempDir(), "memory.vec"), vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if err := idx.Upsert(Entry{ID: "ve-seed", SourceType: SourceMemoryNote, SourceID: "seed", ChunkHash: "h1", Vector: vec, EmbeddingModel: "m"}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := range 200 {
			if err := idx.Upsert(Entry{ID: fmt.Sprintf("ve-%d", i), SourceType: SourceMemoryNote, SourceID: fmt.Sprintf("n%d", i), ChunkHash: "h2", Vector: vec, EmbeddingModel: "m"}); err != nil {
				t.Errorf("Upsert() error: %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			if _, err := idx.Search(vec, 5); err != nil {
				t.Errorf("Search() error: %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for range 50 {
			if err := idx.Flush(); err != nil {
				t.Errorf("Flush() error: %v", err)
				return
			}
		}
	}()
	wg.Wait()
}

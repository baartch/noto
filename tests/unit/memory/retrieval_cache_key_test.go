package memory_test

import (
	"testing"

	"noto/internal/cache"
)

func TestCacheIdentityKey_ChangesWithEmbeddingModel(t *testing.T) {
	a := cache.Identity{ProfileID: "p", Prompt: "prompt", NotesHash: "h", TokenBudget: 1500, EmbeddingModel: "model-a"}.Key()
	b := cache.Identity{ProfileID: "p", Prompt: "prompt", NotesHash: "h", TokenBudget: 1500, EmbeddingModel: "model-b"}.Key()
	if a == b {
		t.Fatalf("expected different keys")
	}
}

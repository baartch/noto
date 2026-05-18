package cache_test

import (
	"testing"

	"noto/internal/cache"
)

func TestIdentityKey_ChangesWithEmbeddingModel(t *testing.T) {
	a := cache.Identity{ProfileID: "p", Prompt: "s", NotesHash: "h", TokenBudget: 100, EmbeddingModel: "m1"}.Key()
	b := cache.Identity{ProfileID: "p", Prompt: "s", NotesHash: "h", TokenBudget: 100, EmbeddingModel: "m2"}.Key()
	if a == b {
		t.Fatalf("expected different keys for different embedding models")
	}
}

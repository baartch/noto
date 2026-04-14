package memory

import (
	"testing"
	"time"

	"noto/internal/store"
)

func TestSelectNotesForContext_RankedOrder(t *testing.T) {
	notes := []*store.MemoryNote{
		{ID: "n1", Content: "Alpha", Importance: 5, CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "n2", Content: "Beta", Importance: 8, CreatedAt: time.Now().Add(-1 * time.Hour)},
		{ID: "n3", Content: "Gamma", Importance: 3, CreatedAt: time.Now().Add(-3 * time.Hour)},
	}
	ordered := SelectNotesForContext(notes, []string{"n3", "n1", "n2"}, 100)
	if len(ordered) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(ordered))
	}
	if ordered[0].ID != "n3" || ordered[1].ID != "n1" || ordered[2].ID != "n2" {
		t.Fatalf("unexpected order: %s, %s, %s", ordered[0].ID, ordered[1].ID, ordered[2].ID)
	}
}

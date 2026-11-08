package vector

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"noto/internal/provider"
	"noto/internal/startup"
	vecfile "noto/internal/vector/file"
)

type progressEmbedder struct{ fail bool }

func (e progressEmbedder) Embed(context.Context, provider.EmbeddingRequest) (*provider.EmbeddingResponse, error) {
	if e.fail {
		return nil, errors.New("private provider error")
	}
	return &provider.EmbeddingResponse{Embedding: []float32{1, 0}}, nil
}

type progressManifest struct{}

func (progressManifest) SetManifestStatusStr(context.Context, string, string) error { return nil }

func TestRebuildReportsProgressAndFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		index := NewFileIndex(filepath.Join(t.TempDir(), "memory.vec"), vecfile.NewBinaryCodec(), nil)
		var events []startup.Event
		rb := NewRebuilder(progressManifest{}, index, "p").WithEmbedder(progressEmbedder{fail}, "model").WithProgress(func(e startup.Event) { events = append(events, e) })
		err := rb.Rebuild(context.Background(), []MemoryNoteRecord{{ID: "a", Content: "private"}, {ID: "b", Content: "private"}})
		if (err != nil) != fail {
			t.Fatalf("Rebuild error = %v, fail = %t", err, fail)
		}
		want := startup.Event{Phase: "rebuild", State: "success", Done: 2, Total: 2}
		if fail {
			want.State, want.Done = "failure", 0
		}
		if len(events) < 2 || events[0] != (startup.Event{Phase: "rebuild", State: "start", Total: 2}) || events[len(events)-1] != want {
			t.Fatalf("events = %#v, want final %#v", events, want)
		}
	}
}

func TestSyncReportsCompletedNotes(t *testing.T) {
	index := NewFileIndex(filepath.Join(t.TempDir(), "memory.vec"), vecfile.NewBinaryCodec(), nil)
	var events []startup.Event
	syncer := NewSyncer(index, "p", progressEmbedder{}, "model").WithProgress(func(e startup.Event) { events = append(events, e) })
	if err := syncer.SyncNotes(context.Background(), []MemoryNoteRecord{{ID: "a", Content: "private"}}); err != nil {
		t.Fatal(err)
	}
	want := startup.Event{Phase: "sync", State: "success", Done: 1, Total: 1}
	if len(events) < 2 || events[0] != (startup.Event{Phase: "sync", State: "start", Total: 1}) || events[len(events)-1] != want {
		t.Fatalf("events = %#v, want final %#v", events, want)
	}
}

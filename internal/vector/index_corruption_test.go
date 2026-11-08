package vector

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vecfile "noto/internal/vector/file"
	"noto/internal/vector/hnsw"
)

func TestFileIndexReloadAfterReplacementAndDeletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.vec")
	index := NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	for _, e := range []Entry{
		{ID: "ve-a", SourceType: SourceMemoryNote, SourceID: "a", Vector: []float32{1, 0}},
		{ID: "ve-b", SourceType: SourceMemoryNote, SourceID: "b", Vector: []float32{0, 1}},
		{ID: "ve-a", SourceType: SourceMemoryNote, SourceID: "a", Vector: []float32{0.8, 0.2}},
	} {
		if err := index.Upsert(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := index.Delete(SourceMemoryNote, "b"); err != nil {
		t.Fatal(err)
	}
	if err := index.Flush(); err != nil {
		t.Fatal(err)
	}
	reloaded := NewFileIndex(path, vecfile.NewBinaryCodec(), nil)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("reload after replacement and deletion: %v", err)
	}
	results, err := reloaded.Search([]float32{0.8, 0.2}, 10)
	if err != nil || len(results) != 1 || results[0].Entry.SourceID != "a" || results[0].Score < 0.99 {
		t.Fatalf("search after reload = %#v, %v; want only replacement a", results, err)
	}
}

func TestFileIndexCorruptionReportsCause(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.vec")
	if err := os.WriteFile(path, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := NewFileIndex(path, vecfile.NewBinaryCodec(), nil).Load()
	if !errors.Is(err, ErrIndexCorrupted) || !strings.Contains(err.Error(), "read magic") {
		t.Fatalf("Load() = %v; want corruption and read magic cause", err)
	}
}

func TestFileIndexRebuildDiscardsOldGraph(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.vec")
	index := NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	if err := index.Upsert(Entry{ID: "ve-a", SourceType: SourceMemoryNote, SourceID: "a", Vector: []float32{1, 0}}); err != nil {
		t.Fatal(err)
	}
	if err := index.Rebuild([]Entry{{ID: "ve-a", SourceType: SourceMemoryNote, SourceID: "a", Vector: []float32{0, 1}}}); err != nil {
		t.Fatal(err)
	}
	results, err := index.Search([]float32{1, 0}, 1)
	if err != nil || len(results) != 1 || results[0].Score != 0 {
		t.Fatalf("search after rebuild = %#v, %v; want score 0", results, err)
	}
}

func TestFileIndexLoadDoesNotDiscardUnflushedUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.vec")
	index := NewFileIndex(path, vecfile.NewBinaryCodec(), nil)
	if err := index.Upsert(Entry{ID: "ve-a", SourceType: SourceMemoryNote, SourceID: "a", Vector: []float32{1, 0}}); err != nil {
		t.Fatal(err)
	}
	if err := index.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := index.Upsert(Entry{ID: "ve-b", SourceType: SourceMemoryNote, SourceID: "b", Vector: []float32{0, 1}}); err != nil {
		t.Fatal(err)
	}
	if err := index.Load(); err != nil {
		t.Fatal(err)
	}
	if _, ok := index.VectorRefFor(SourceMemoryNote, "b"); !ok {
		t.Fatal("Load discarded an unflushed update")
	}
}

func TestFileIndexGraphUsesUpdatedVector(t *testing.T) {
	index := NewFileIndex(filepath.Join(t.TempDir(), "memory.vec"), vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	if err := index.Upsert(Entry{ID: "ve-a", SourceType: SourceMemoryNote, SourceID: "a", Vector: []float32{1, 0}}); err != nil {
		t.Fatal(err)
	}
	if err := index.Upsert(Entry{ID: "ve-a", SourceType: SourceMemoryNote, SourceID: "a", Vector: []float32{0, 1}}); err != nil {
		t.Fatal(err)
	}
	results, err := index.Search([]float32{1, 0}, 1)
	if err != nil || len(results) != 1 || results[0].Score != 0 {
		t.Fatalf("search after update = %#v, %v; want score 0", results, err)
	}
}

func TestFileIndexRejectsImpossibleGraphSize(t *testing.T) {
	var data bytes.Buffer
	codec := vecfile.NewBinaryCodec()
	if err := codec.WriteHeader(&data, vecfile.Header{}); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&data, binary.LittleEndian, ^uint64(0)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "memory.vec")
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	err := NewFileIndex(path, codec, nil).Load()
	if !errors.Is(err, ErrIndexCorrupted) {
		t.Fatalf("Load() = %v; want corruption error", err)
	}
}

func TestFileIndexRejectsInvalidVectorReference(t *testing.T) {
	var data bytes.Buffer
	codec := vecfile.NewBinaryCodec()
	if err := codec.WriteHeader(&data, vecfile.Header{EmbeddingDim: 2, EntryCount: 1}); err != nil {
		t.Fatal(err)
	}
	if err := codec.WriteVectors(&data, []float32{1, 0}, 2); err != nil {
		t.Fatal(err)
	}
	if err := codec.WriteEntries(&data, []vecfile.EntryRecord{{SourceType: "memory_note", SourceID: "a", VectorRef: "-1"}}); err != nil {
		t.Fatal(err)
	}
	if err := codec.WriteGraph(&data, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "memory.vec")
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	err := NewFileIndex(path, codec, nil).Load()
	if !errors.Is(err, ErrIndexCorrupted) {
		t.Fatalf("Load() = %v; want corruption error", err)
	}
}

func TestFileIndexFailedRebuildDoesNotHideCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.vec")
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewFileIndex(path, vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	err := index.Rebuild([]Entry{
		{ID: "ve-a", SourceType: SourceMemoryNote, SourceID: "a", Vector: []float32{1, 0}},
		{ID: "ve-b", SourceType: SourceMemoryNote, SourceID: "b", Vector: []float32{1, 0, 0}},
	})
	if err == nil {
		t.Fatal("expected rebuild failure")
	}
	if err := index.Load(); !errors.Is(err, ErrIndexCorrupted) {
		t.Fatalf("Load after failed rebuild = %v; want original corruption error", err)
	}
}

func TestFileIndexDeleteRemovesGraphNode(t *testing.T) {
	index := NewFileIndex(filepath.Join(t.TempDir(), "memory.vec"), vecfile.NewBinaryCodec(), hnsw.NewSimpleGraph(0))
	for _, e := range []Entry{
		{ID: "ve-a", SourceType: SourceMemoryNote, SourceID: "a", Vector: []float32{0.8, 0.2}},
		{ID: "ve-b", SourceType: SourceMemoryNote, SourceID: "b", Vector: []float32{1, 0}},
		{ID: "ve-c", SourceType: SourceMemoryNote, SourceID: "c", Vector: []float32{0, 1}},
	} {
		if err := index.Upsert(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := index.Delete(SourceMemoryNote, "b"); err != nil {
		t.Fatal(err)
	}
	results, err := index.Search([]float32{1, 0}, 2)
	if err != nil || len(results) != 2 {
		t.Fatalf("search after delete = %#v, %v; want both remaining notes", results, err)
	}
}

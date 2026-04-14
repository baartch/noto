package integration

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"noto/internal/commands"
	"noto/internal/profile"
	"noto/internal/store"
)

func TestMemoryReviewList_ShowsSourceAndReason(t *testing.T) {
	db, closeDB := tempDB(t)
	defer closeDB()
	ctx := context.Background()

	repo := store.NewProfileRepo(db)
	svc := profile.NewService(repo)
	p, err := svc.Create(ctx, "Review")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	noteRepo := store.NewMemoryNoteRepo(db)
	note := &store.MemoryNote{
		ID:               "mn-review",
		ProfileID:        p.ID,
		Category:         store.CategoryFact,
		Content:          "User prefers short answers",
		Importance:       8,
		SourceMessageIDs: "[\"msg-1\",\"msg-2\"]",
	}
	if err := noteRepo.Create(ctx, note); err != nil {
		t.Fatalf("create note: %v", err)
	}

	registry := commands.NewRegistry()
	if err := commands.RegisterMemoryCommands(registry); err != nil {
		t.Fatalf("register memory: %v", err)
	}

	buf := &bytes.Buffer{}
	execCtx := &commands.ExecContext{ProfileID: p.ID, ProfileSlug: p.Slug, DB: db, Output: buf}
	cmd, ok := registry.Lookup("memory list")
	if !ok {
		t.Fatalf("missing memory list command")
	}
	if err := cmd.Handler(execCtx, nil); err != nil {
		t.Fatalf("memory list: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "User prefers short answers") {
		t.Fatalf("expected note content in output")
	}
	if !strings.Contains(out, "sources:") {
		t.Fatalf("expected sources in output")
	}
	if !strings.Contains(out, "importance: 8") {
		t.Fatalf("expected rationale in output")
	}
}

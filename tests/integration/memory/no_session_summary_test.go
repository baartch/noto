package integration

import (
	"context"
	"strings"
	"testing"

	"noto/internal/memory"
	"noto/internal/profile"
	"noto/internal/store"
	"noto/tests/integration/testutil"
)

func TestAssembledPrompt_ExcludesConversationSummaries(t *testing.T) {
	db, closeDB := testutil.TempDB(t)
	defer closeDB()
	ctx := context.Background()

	p, _ := profile.NewService(store.NewProfileRepo(db)).Create(ctx, "NoSummary")
	summaryRepo := store.NewSessionSummaryRepo(db)
	if err := summaryRepo.Create(ctx, &store.SessionSummary{ID: "ss-1", ProfileID: p.ID, SummaryText: "legacy session summary", OpenLoops: "[]", NextActions: "[]"}); err != nil {
		t.Fatalf("create session summary: %v", err)
	}

	assembled := memory.AssemblePrompt("system", "", "## Raw Notes\n- [fact] current note")
	if strings.Contains(assembled, "legacy session summary") {
		t.Fatalf("did not expect legacy session summary in assembled prompt: %q", assembled)
	}
	if strings.Contains(assembled, "Previous Session Summary") {
		t.Fatalf("did not expect previous session section in assembled prompt: %q", assembled)
	}
}

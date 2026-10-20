package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestHistoryCursorSameTimestampAcrossConversations(t *testing.T) {
	db := openHistoryTestDB(t)
	ctx := context.Background()
	conversations := NewConversationRepo(db)
	messages := NewMessageRepo(db)
	for _, conversation := range []*Conversation{
		{ID: "c1", ProfileID: "p1", Status: ConversationArchived},
		{ID: "c2", ProfileID: "p1", Status: ConversationArchived},
		{ID: "c3", ProfileID: "p2", Status: ConversationArchived},
	} {
		if err := conversations.Create(ctx, conversation); err != nil {
			t.Fatal(err)
		}
	}

	createdAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	want := []string{"p1-0", "p1-1", "p1-2", "p1-3", "p1-4", "p1-5", "p1-6"}
	for i, id := range want {
		conversationID := "c1"
		if i%2 != 0 {
			conversationID = "c2"
		}
		createHistoryTestMessage(t, db, messages, id, conversationID, RoleUser, createdAt)
		if i == 1 || i == 4 {
			createHistoryTestMessage(t, db, messages, "p2-"+id, "c3", RoleUser, createdAt)
		}
	}

	var cursor *MessageHistoryCursor
	var got []string
	for pageNumber := range 4 {
		page, err := messages.ListHistoryByProfile(ctx, "p1", cursor, 3)
		if err != nil {
			t.Fatalf("ListHistoryByProfile page %d: %v", pageNumber, err)
		}
		pageIDs := make([]string, 0, len(page.Messages))
		for _, message := range page.Messages {
			pageIDs = append(pageIDs, message.ID)
			if message.ConversationStartedAt.IsZero() {
				t.Errorf("message %s has no conversation start time", message.ID)
			}
		}
		got = append(pageIDs, got...)
		cursor = page.Next
		if cursor == nil {
			break
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paged IDs = %v, want %v", got, want)
	}
	seen := make(map[string]bool, len(got))
	for _, id := range got {
		if seen[id] {
			t.Fatalf("duplicate message %q across pages", id)
		}
		seen[id] = true
	}
}

func TestHistoryCursorEmptyAndExactPages(t *testing.T) {
	db := openHistoryTestDB(t)
	ctx := context.Background()
	repo := NewMessageRepo(db)
	page, err := repo.ListHistoryByProfile(ctx, "p1", nil, 3)
	if err != nil || len(page.Messages) != 0 || page.Next != nil {
		t.Fatalf("empty page = (%d, %v, %v), want (0, nil, nil)", len(page.Messages), page.Next, err)
	}

	if err := NewConversationRepo(db).Create(ctx, &Conversation{ID: "c1", ProfileID: "p1", Status: ConversationArchived}); err != nil {
		t.Fatal(err)
	}
	for i := range 6 {
		createHistoryTestMessage(t, db, repo, string(rune('a'+i)), "c1", RoleUser, time.Date(2026, 1, 2, 3, 4, i, 0, time.UTC))
	}
	first, err := repo.ListHistoryByProfile(ctx, "p1", nil, 3)
	if err != nil || len(first.Messages) != 3 || first.Next == nil {
		t.Fatalf("first exact page = (%d, %v, %v), want 3 rows and cursor", len(first.Messages), first.Next, err)
	}
	second, err := repo.ListHistoryByProfile(ctx, "p1", first.Next, 3)
	if err != nil || len(second.Messages) != 3 || second.Next != nil {
		t.Fatalf("second exact page = (%d, %v, %v), want 3 rows and no cursor", len(second.Messages), second.Next, err)
	}
}

func TestHistoryCursorIgnoresNewerMessagesInsertedBetweenPages(t *testing.T) {
	db := openHistoryTestDB(t)
	ctx := context.Background()
	repo := NewMessageRepo(db)
	if err := NewConversationRepo(db).Create(ctx, &Conversation{ID: "c1", ProfileID: "p1", Status: ConversationArchived}); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, id := range []string{"m1", "m2", "m3", "m4"} {
		createHistoryTestMessage(t, db, repo, id, "c1", RoleUser, createdAt)
	}
	latest, err := repo.ListHistoryByProfile(ctx, "p1", nil, 2)
	if err != nil || latest.Next == nil {
		t.Fatalf("latest page = (%v, %v), want cursor", err, latest.Next)
	}
	createHistoryTestMessage(t, db, repo, "m5", "c1", RoleUser, createdAt)
	older, err := repo.ListHistoryByProfile(ctx, "p1", latest.Next, 2)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{older.Messages[0].ID, older.Messages[1].ID}
	if want := []string{"m1", "m2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("older page after insert = %v, want %v", got, want)
	}
}

func openHistoryTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenForTesting(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func createHistoryTestMessage(t *testing.T, db *DB, repo *MessageRepo, id, conversationID string, role MessageRole, createdAt time.Time) {
	t.Helper()
	if err := repo.Create(context.Background(), &Message{ID: id, ConversationID: conversationID, Role: role, Content: id}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE messages SET created_at = ? WHERE id = ?`, createdAt.UTC(), id); err != nil {
		t.Fatal(err)
	}
}

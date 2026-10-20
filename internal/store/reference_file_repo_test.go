//go:build !production

package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestReferenceFileRepoRoundTripAndReconcile(t *testing.T) {
	db, err := OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewReferenceFileRepo(db)
	ctx := context.Background()
	first := ReferenceFile{ID: "rf-1", ProfileID: "p1", RelativePath: "identity.md", MediaType: "text/markdown", Fingerprint: "sha-a", SummaryFingerprint: "sha-a", Summary: "Artist identity", Status: ReferenceFileReady, Size: 12, ModTimeNS: 30}
	second := ReferenceFile{ID: "rf-2", ProfileID: "p1", RelativePath: "z/voice.txt", MediaType: "text/plain", Fingerprint: "sha-b", SummaryFingerprint: "sha-b", Summary: "Voice guide", Status: ReferenceFileReady}
	otherProfile := ReferenceFile{ID: "rf-3", ProfileID: "p2", RelativePath: "identity.md", Status: ReferenceFileFailed, LastError: "unreadable"}
	for _, file := range []ReferenceFile{second, first, otherProfile} {
		if err := repo.Upsert(ctx, file); err != nil {
			t.Fatal(err)
		}
	}

	first.Summary = "Updated description"
	first.Fingerprint = "sha-c"
	if err := repo.Upsert(ctx, first); err != nil {
		t.Fatal(err)
	}
	files, err := repo.List(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].RelativePath != "identity.md" || files[1].RelativePath != "z/voice.txt" {
		t.Fatalf("files not isolated or deterministically ordered: %#v", files)
	}
	if files[0].ID != "rf-1" || files[0].SummaryFingerprint != "sha-a" || files[0].Summary != "Updated description" {
		t.Fatalf("upsert did not preserve ID and stale-summary marker: %#v", files[0])
	}
	if _, err := repo.DeleteMissing(ctx, "p1", []string{"z/voice.txt"}); err != nil {
		t.Fatal(err)
	}
	files, err = repo.List(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].RelativePath != "z/voice.txt" {
		t.Fatalf("DeleteMissing result = %#v", files)
	}
	if _, err := repo.DeleteMissing(ctx, "p1", nil); err != nil {
		t.Fatal(err)
	}
	files, err = repo.List(ctx, "p1")
	if err != nil || len(files) != 0 {
		t.Fatalf("empty successful scan should remove profile rows: %#v, %v", files, err)
	}
	other, err := repo.List(ctx, "p2")
	if err != nil || !reflect.DeepEqual([]string{other[0].ID}, []string{"rf-3"}) {
		t.Fatalf("other profile was affected: %#v, %v", other, err)
	}
}

func farFuture() time.Time { return time.Now().Add(24 * time.Hour) }

func TestMessageReferenceInfluenceRoundTrips(t *testing.T) {
	db, err := OpenForTesting(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := NewConversationRepo(db).Create(ctx, &Conversation{ID: "conv", ProfileID: "p1", Status: ConversationArchived}); err != nil {
		t.Fatal(err)
	}
	repo := NewMessageRepo(db)
	for i, marked := range []bool{false, true} {
		if err := repo.Create(ctx, &Message{ID: []string{"m1", "m2"}[i], ConversationID: "conv", Role: RoleAssistant, Content: "reply", ReferenceInfluenced: marked}); err != nil {
			t.Fatal(err)
		}
	}
	checks := []struct {
		name string
		read func() ([]*Message, error)
	}{
		{"conversation", func() ([]*Message, error) { return repo.ListByConversation(ctx, "conv") }},
		{"recent conversation", func() ([]*Message, error) { return repo.ListRecentByConversation(ctx, "conv", 2) }},
		{"recent profile", func() ([]*Message, error) { return repo.ListRecentByProfile(ctx, "p1", 2) }},
		{"older conversation", func() ([]*Message, error) { return repo.ListOlderByConversationBefore(ctx, "conv", farFuture(), 10) }},
		{"older profile", func() ([]*Message, error) { return repo.ListOlderByProfileBefore(ctx, "p1", farFuture(), 10) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			messages, err := check.read()
			if err != nil {
				t.Fatal(err)
			}
			if len(messages) != 2 {
				t.Fatalf("provenance not preserved: %#v", messages)
			}
			got := map[string]bool{messages[0].ID: messages[0].ReferenceInfluenced, messages[1].ID: messages[1].ReferenceInfluenced}
			if got["m1"] || !got["m2"] {
				t.Fatalf("provenance not preserved: %#v", got)
			}
		})
	}
}

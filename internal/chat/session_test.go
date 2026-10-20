package chat

import (
	"reflect"
	"testing"

	"noto/internal/reference"
	"noto/internal/store"
)

func TestSessionPendingNotesSnapshot(t *testing.T) {
	s := &Session{}
	if got := s.PendingNotes(); got != 0 {
		t.Fatalf("initial PendingNotes() = %d, want 0", got)
	}
	s.markNotesPending()
	if got := s.PendingNotes(); got != 1 {
		t.Fatalf("PendingNotes() after pending = %d, want 1", got)
	}
	s.markNotesDone(1)
	if got := s.PendingNotes(); got != 0 {
		t.Fatalf("PendingNotes() after completion = %d, want 0", got)
	}
}

func TestReferenceInfluencePropagatesFromCatalogAttachmentsAndHistory(t *testing.T) {
	markedHistory := []*store.Message{{Role: store.RoleAssistant, ReferenceInfluenced: true}}
	cases := []struct {
		name        string
		catalog     string
		attachments []reference.Attachment
		history     []*store.Message
		want        bool
	}{
		{name: "ordinary turn", want: false},
		{name: "catalog summary present", catalog: "[]", want: true},
		{name: "raw attachment present", attachments: []reference.Attachment{{Text: "body"}}, want: true},
		{name: "restored marked reply", history: markedHistory, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := referenceInfluenced(tc.history, tc.catalog, tc.attachments); got != tc.want {
				t.Fatalf("referenceInfluenced() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestMemoryExtractionInputExcludesAssistantWhenReferenceInfluenced(t *testing.T) {
	ids, user, assistant := memoryExtractionInput("user-id", "assistant-id", "explicit user fact", "document-derived assistant fact", true)
	if !reflect.DeepEqual(ids, []string{"user-id"}) || user != "explicit user fact" || assistant != "" {
		t.Fatalf("influenced extraction input = %#v, %q, %q", ids, user, assistant)
	}
	ids, user, assistant = memoryExtractionInput("user-id", "assistant-id", "user", "ordinary assistant", false)
	if !reflect.DeepEqual(ids, []string{"user-id", "assistant-id"}) || user != "user" || assistant != "ordinary assistant" {
		t.Fatalf("ordinary extraction input = %#v, %q, %q", ids, user, assistant)
	}
}

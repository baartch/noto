package memory

import (
	"context"
	"fmt"
	"time"

	"noto/internal/store"
)

// ExtractionResult holds the notes extracted from a set of messages.
type ExtractionResult struct {
	Notes []*store.MemoryNote
}

// Extractor extracts memory notes from conversation messages.
// The current implementation uses heuristic extraction; a production version
// would call the LLM to identify facts, progress, blockers, and action items.
type Extractor struct {
	noteRepo *store.MemoryNoteRepo
}

// NewExtractor creates a new Extractor.
func NewExtractor(noteRepo *store.MemoryNoteRepo) *Extractor {
	return &Extractor{noteRepo: noteRepo}
}

// Extract analyses messages and persists any identified memory notes.
func (e *Extractor) Extract(ctx context.Context, profileID, conversationID string, messages []*store.Message) (*ExtractionResult, error) {
	var notes []*store.MemoryNote

	for _, msg := range messages {
		if msg.Role != store.RoleAssistant {
			continue
		}
		// Heuristic: persist assistant messages longer than 100 chars as facts.
		if len(msg.Content) > 100 {
			note := &store.MemoryNote{
				ID:               fmt.Sprintf("mn-%x", time.Now().UnixNano()),
				ProfileID:        profileID,
				ConversationID:   conversationID,
				Category:         store.CategoryFact,
				Content:          summarize(msg.Content, 500),
				Importance:       5,
				SourceMessageIDs: fmt.Sprintf(`["%s"]`, msg.ID),
			}
			if err := e.noteRepo.Create(ctx, note); err != nil {
				return nil, fmt.Errorf("memory: create note: %w", err)
			}
			notes = append(notes, note)
		}
	}

	return &ExtractionResult{Notes: notes}, nil
}

// summarize returns the first n runes of s (UTF-8 safe).
func summarize(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

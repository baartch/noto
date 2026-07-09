package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"noto/internal/store"
)

// StoreCandidate persists a new note from the candidate.
func StoreCandidate(ctx context.Context, repo *store.MemoryNoteRepo, profileID, conversationID string, candidate NoteCandidate, sourceMessageIDs []string) (*store.MemoryNote, error) {
	importance := normalizeImportance(candidate)

	sourceIDs, err := normalizeSourceMessageIDs(sourceMessageIDs)
	if err != nil {
		return nil, err
	}

	cat := normalizeCategory(candidate.Category)
	note := &store.MemoryNote{
		ID:               fmt.Sprintf("mn-%x", time.Now().UnixNano()),
		ProfileID:        profileID,
		ConversationID:   conversationID,
		Category:         cat,
		Content:          candidate.Content,
		Importance:       importance,
		SourceMessageIDs: sourceIDs,
	}
	if err := repo.Create(ctx, note); err != nil {
		return nil, fmt.Errorf("memory: save note: %w", err)
	}
	return note, nil
}

// LinkCandidateContext links new context to an existing note when a duplicate is detected.
func LinkCandidateContext(ctx context.Context, repo *store.MemoryNoteRepo, note *store.MemoryNote, candidate NoteCandidate, sourceMessageIDs []string) error {
	if note == nil {
		return nil
	}
	merged, changed, err := mergeSourceMessageIDs(note.SourceMessageIDs, sourceMessageIDs)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	note.SourceMessageIDs = merged
	return repo.Update(ctx, note)
}

// UpdateCandidateAsDuplicate merges candidate content into the existing note and links context.
// Unlike LinkCandidateContext which only merges source IDs, this updates the note's
// content, importance, and category to reflect the refined information.
func UpdateCandidateAsDuplicate(ctx context.Context, repo *store.MemoryNoteRepo, note *store.MemoryNote, candidate NoteCandidate, sourceMessageIDs []string) error {
	if note == nil {
		return nil
	}
	if candidate.Content != "" && candidate.Content != note.Content {
		note.Content = candidate.Content
	}
	if candidate.Importance > 0 {
		note.Importance = normalizeImportance(candidate)
	}
	cat := normalizeCategory(candidate.Category)
	if cat != note.Category {
		note.Category = cat
	}
	merged, _, err := mergeSourceMessageIDs(note.SourceMessageIDs, sourceMessageIDs)
	if err != nil {
		return err
	}
	note.SourceMessageIDs = merged
	return repo.Update(ctx, note)
}

func normalizeCategory(raw string) store.MemoryCategory {
	cat := store.MemoryCategory(raw)
	switch cat {
	case store.CategoryFact, store.CategoryProgress, store.CategoryBlocker, store.CategoryActionItem, store.CategoryOther:
		return cat
	default:
		return store.CategoryOther
	}
}

func normalizeImportance(candidate NoteCandidate) int {
	importance := candidate.Importance
	if importance <= 0 {
		importance = candidate.ValueScore.Total
	}
	if importance <= 0 {
		importance = DefaultValueScore
	}
	if importance < 1 {
		return 1
	}
	if importance > 10 {
		return 10
	}
	return importance
}

func normalizeSourceMessageIDs(ids []string) (string, error) {
	if len(ids) == 0 {
		return "[]", nil
	}
	unique := make(map[string]struct{}, len(ids))
	ordered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := unique[id]; ok {
			continue
		}
		unique[id] = struct{}{}
		ordered = append(ordered, id)
	}
	data, err := json.Marshal(ordered)
	if err != nil {
		return "[]", fmt.Errorf("memory: marshal source ids: %w", err)
	}
	return string(data), nil
}

func mergeSourceMessageIDs(existingJSON string, newIDs []string) (string, bool, error) {
	if len(newIDs) == 0 {
		return existingJSON, false, nil
	}
	var existing []string
	if existingJSON != "" {
		if err := json.Unmarshal([]byte(existingJSON), &existing); err != nil {
			return "", false, fmt.Errorf("memory: parse source ids: %w", err)
		}
	}
	unique := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		if id == "" {
			continue
		}
		unique[id] = struct{}{}
	}
	changed := false
	for _, id := range newIDs {
		if id == "" {
			continue
		}
		if _, ok := unique[id]; ok {
			continue
		}
		unique[id] = struct{}{}
		existing = append(existing, id)
		changed = true
	}
	if !changed {
		return existingJSON, false, nil
	}
	data, err := json.Marshal(existing)
	if err != nil {
		return "", false, fmt.Errorf("memory: marshal source ids: %w", err)
	}
	return string(data), true, nil
}

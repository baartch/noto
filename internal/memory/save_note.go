package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"noto/internal/store"
	"noto/internal/vector"
)

// SaveNoteInput is a validated explicit note save request (e.g. from a
// save_note tool call).
type SaveNoteInput struct {
	Content    string // required, non-empty after TrimSpace
	Category   string // "" → "fact"; must be a known category else error
	Importance int    // 0 → default; clamped to 1–10
}

// SaveNoteResult reports the outcome of an explicit note save.
type SaveNoteResult struct {
	Note    *store.MemoryNote
	Updated bool // true when merged into an existing duplicate
}

// SaveNoteExplicit persists an explicit note save request. Unlike extracted
// notes it bypasses the value-score gate (MinValueScore) — the requester
// already decided the note is worth keeping — but not deduplication or
// validation. When deduper is nil, a text-overlap fallback (jaccard
// similarity, same threshold as the extractor) merges into the best match.
func SaveNoteExplicit(
	ctx context.Context,
	noteRepo *store.MemoryNoteRepo,
	deduper vector.Deduper,
	logHook CaptureLogHook,
	profileID, conversationID string,
	in SaveNoteInput,
	sourceMessageIDs []string,
) (*SaveNoteResult, error) {
	if logHook == nil {
		logHook = NoopCaptureLogHook{}
	}
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, errors.New("memory: save note: empty content")
	}
	category := in.Category
	if category == "" {
		category = string(store.CategoryFact)
	}
	if !isKnownCategory(store.MemoryCategory(category)) {
		return nil, fmt.Errorf("memory: save note: invalid category %q", in.Category)
	}

	candidate := EvaluateCandidate(content, in.Importance, []string{content})
	candidate.Category = category
	candidate.Importance = in.Importance
	logHook.CandidateScored(candidate)

	// Dedup first: vector deduper when available, text-overlap fallback when not.
	if deduper != nil {
		res, err := deduper.CheckDuplicate(ctx, profileID, candidate.Content)
		if err == nil && res.IsDuplicate {
			note, err := noteRepo.GetByID(ctx, res.MatchID)
			if err == nil && note.ProfileID == profileID {
				if err := UpdateCandidateAsDuplicate(ctx, noteRepo, note, candidate, sourceMessageIDs); err != nil {
					logHook.NoteStorageFailed(candidate, err)
					return nil, err
				}
				logHook.DuplicateDetected(candidate, note.ID)
				markSummariesStaleForNote(ctx, noteRepo, profileID, note)
				return &SaveNoteResult{Note: note, Updated: true}, nil
			}
		}
	} else if matchID := findTextDuplicate(ctx, noteRepo, profileID, candidate.Content); matchID != "" {
		note, err := noteRepo.GetByID(ctx, matchID)
		if err == nil {
			if err := UpdateCandidateAsDuplicate(ctx, noteRepo, note, candidate, sourceMessageIDs); err != nil {
				logHook.NoteStorageFailed(candidate, err)
				return nil, err
			}
			logHook.DuplicateDetected(candidate, note.ID)
			markSummariesStaleForNote(ctx, noteRepo, profileID, note)
			return &SaveNoteResult{Note: note, Updated: true}, nil
		}
	}

	note, err := StoreCandidate(ctx, noteRepo, profileID, conversationID, candidate, sourceMessageIDs)
	if err != nil {
		logHook.NoteStorageFailed(candidate, err)
		return nil, err
	}
	logHook.NoteStored(candidate, note.ID)
	markSummariesStaleForNote(ctx, noteRepo, profileID, note)
	return &SaveNoteResult{Note: note, Updated: false}, nil
}

// markSummariesStaleForNote marks weekly/monthly summaries covering the note
// stale, mirroring the extractor's post-storage step. Non-fatal by design.
func markSummariesStaleForNote(ctx context.Context, noteRepo *store.MemoryNoteRepo, profileID string, note *store.MemoryNote) {
	if noteRepo == nil || noteRepo.DB() == nil {
		return
	}
	_ = NewSummaryRollupBuilder(noteRepo, store.NewMemorySummaryRepo(noteRepo.DB())).MarkCoveredSummariesStale(ctx, profileID, note.CreatedAt)
}

func isKnownCategory(cat store.MemoryCategory) bool {
	switch cat {
	case store.CategoryFact, store.CategoryProgress, store.CategoryBlocker, store.CategoryActionItem, store.CategoryOther:
		return true
	default:
		return false
	}
}

// findTextDuplicate returns the ID of the most similar existing note when
// word overlap reaches textDedupThreshold, else "".
func findTextDuplicate(ctx context.Context, noteRepo *store.MemoryNoteRepo, profileID, content string) string {
	existing, err := noteRepo.ListByProfile(ctx, profileID)
	if err != nil {
		return ""
	}
	itemWords := tokenize(content)
	if len(itemWords) == 0 {
		return ""
	}
	var bestID string
	var bestScore float64
	for _, note := range existing {
		noteWords := tokenize(note.Content)
		if len(noteWords) == 0 {
			continue
		}
		score := jaccardSimilarity(itemWords, noteWords)
		if score > bestScore {
			bestScore = score
			bestID = note.ID
		}
	}
	if bestID != "" && bestScore >= textDedupThreshold {
		return bestID
	}
	return ""
}

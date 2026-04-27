package memory

import (
	"context"

	"noto/internal/store"
	"noto/internal/vector"
)

// Processor orchestrates scoring, deduplication, and storage for extracted notes.
type Processor struct {
	noteRepo *store.MemoryNoteRepo
	deduper  vector.Deduper
	logHook  CaptureLogHook
}

// NewProcessor creates a Processor.
func NewProcessor(noteRepo *store.MemoryNoteRepo, deduper vector.Deduper, logHook CaptureLogHook) *Processor {
	if logHook == nil {
		logHook = NoopCaptureLogHook{}
	}
	return &Processor{noteRepo: noteRepo, deduper: deduper, logHook: logHook}
}

// Process runs scoring, deduplication, and storage for extracted items.
func (p *Processor) Process(
	ctx context.Context,
	profileID string,
	conversationID string,
	items []extractedItem,
	sourceMessageIDs []string,
) ([]*store.MemoryNote, int, error) {
	var notes []*store.MemoryNote
	updated := 0

	for _, item := range items {
		if item.Action != "" && item.Action != "add" {
			continue
		}
		candidate := EvaluateCandidate(item.Content, item.Importance, []string{item.Content})
		candidate.Category = item.Category
		candidate.Importance = item.Importance
		p.logHook.CandidateScored(candidate)
		if candidate.ValueScore.Total < MinValueScore {
			continue
		}
		if p.deduper != nil {
			res, err := p.deduper.CheckDuplicate(ctx, profileID, candidate.Content)
			if err == nil && res.IsDuplicate {
				candidate.DuplicateOf = res.MatchID
				p.logHook.DuplicateDetected(candidate, res.MatchID)
				note, err := p.noteRepo.GetByID(ctx, res.MatchID)
				if err == nil {
					if err := LinkCandidateContext(ctx, p.noteRepo, note, candidate, sourceMessageIDs); err != nil {
						p.logHook.NoteStorageFailed(candidate, err)
						return notes, updated, err
					}
					updated++
				}
				continue
			}
		}

		note, err := StoreCandidate(ctx, p.noteRepo, profileID, conversationID, candidate, sourceMessageIDs)
		if err != nil {
			p.logHook.NoteStorageFailed(candidate, err)
			return notes, updated, err
		}
		p.logHook.NoteStored(candidate, note.ID)
		notes = append(notes, note)
	}

	return notes, updated, nil
}

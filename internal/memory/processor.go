package memory

import (
	"context"
	"time"

	"noto/internal/store"
	"noto/internal/vector"
)

// Processor orchestrates scoring, deduplication, and storage for extracted notes.
type Processor struct {
	noteRepo       *store.MemoryNoteRepo
	summaryBuilder *SummaryRollupBuilder
	deduper        vector.Deduper
	logHook        CaptureLogHook
	noteMaxAgeDays int
}

// NewProcessor creates a Processor.
func NewProcessor(noteRepo *store.MemoryNoteRepo, deduper vector.Deduper, logHook CaptureLogHook) *Processor {
	if logHook == nil {
		logHook = NoopCaptureLogHook{}
	}
	return &Processor{noteRepo: noteRepo, deduper: deduper, logHook: logHook}
}

// WithSummaryRollups configures summary stale-marking support for changed notes.
func (p *Processor) WithSummaryRollups(builder *SummaryRollupBuilder) *Processor {
	p.summaryBuilder = builder
	return p
}

// WithNoteMaxAgeDays limits dedup updates to notes created within the given number of days.
// 0 or negative means no limit (all notes are eligible).
func (p *Processor) WithNoteMaxAgeDays(days int) *Processor {
	p.noteMaxAgeDays = days
	return p
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
		if item.Action != "" && item.Action != "add" && item.Action != "update" {
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
				note, err := p.noteRepo.GetByID(ctx, res.MatchID)
				if err == nil {
					// Age guard: only update if note is within the age window
					withinWindow := p.noteMaxAgeDays <= 0
					if !withinWindow {
						cutoff := time.Now().Add(-time.Duration(p.noteMaxAgeDays) * 24 * time.Hour)
						withinWindow = !note.CreatedAt.Before(cutoff)
					}
					if withinWindow {
						candidate.DuplicateOf = res.MatchID
						p.logHook.DuplicateDetected(candidate, res.MatchID)
						if err := UpdateCandidateAsDuplicate(ctx, p.noteRepo, note, candidate, sourceMessageIDs); err != nil {
							p.logHook.NoteStorageFailed(candidate, err)
							return notes, updated, err
						}
						if p.summaryBuilder != nil {
							_ = p.summaryBuilder.MarkCoveredSummariesStale(ctx, profileID, note.CreatedAt)
						}
						updated++
						continue
					}
				}
			}
		}

		note, err := StoreCandidate(ctx, p.noteRepo, profileID, conversationID, candidate, sourceMessageIDs)
		if err != nil {
			p.logHook.NoteStorageFailed(candidate, err)
			return notes, updated, err
		}
		if p.summaryBuilder != nil {
			_ = p.summaryBuilder.MarkCoveredSummariesStale(ctx, profileID, note.CreatedAt)
		}
		p.logHook.NoteStored(candidate, note.ID)
		notes = append(notes, note)
	}

	return notes, updated, nil
}

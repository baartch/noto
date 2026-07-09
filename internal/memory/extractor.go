package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"noto/internal/config"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/vector"
)

const textDedupThreshold = 0.45

// ExtractionResult holds the notes extracted from a single exchange.
type ExtractionResult struct {
	Notes        []*store.MemoryNote
	UpdatedNotes []*store.MemoryNote
	Updated      int
	Merged       int
}

// extractionResponse is the JSON shape the LLM returns for an extraction.
type extractionResponse struct {
	HasNewInfo bool            `json:"has_new_info"`
	Confidence float64         `json:"confidence"`
	Notes      []extractedItem `json:"notes"`
}

// extractedItem is the JSON shape the LLM returns per note.
type extractedItem struct {
	Action     string   `json:"action"`    // add | update | merge
	TargetID   string   `json:"target_id"` // required when action=update
	Category   string   `json:"category"`  // fact | progress | blocker | action_item | other
	Content    string   `json:"content"`
	Importance int      `json:"importance"` // 1-10
	mergeIDs   []string // set by reconcileAdditions; not from LLM
}

// CacheInvalidator invalidates cached memory retrieval context.
type CacheInvalidator interface {
	InvalidateAll(ctx context.Context, profileID string) error
}

// Extractor extracts memory notes using the LLM and persists them to SQLite.
type Extractor struct {
	noteRepo       *store.MemoryNoteRepo
	adapter        provider.Adapter // nil disables extraction
	invalidator    CacheInvalidator
	deduper        vector.Deduper
	logHook        CaptureLogHook
	onUsage        func(provider.Usage)
	noteMaxAgeDays int
}

// NewExtractor creates an Extractor. Pass nil adapter to disable LLM extraction.
func NewExtractor(noteRepo *store.MemoryNoteRepo, adapter provider.Adapter, invalidator CacheInvalidator) *Extractor {
	return &Extractor{noteRepo: noteRepo, adapter: adapter, invalidator: invalidator, logHook: NoopCaptureLogHook{}}
}

// WithDeduper configures the deduper.
func (e *Extractor) WithDeduper(deduper vector.Deduper) *Extractor {
	e.deduper = deduper
	return e
}

// WithLogHook configures capture logging.
func (e *Extractor) WithLogHook(hook CaptureLogHook) *Extractor {
	if hook != nil {
		e.logHook = hook
	}
	return e
}

// WithUsageHook configures usage callback for extractor model calls.
func (e *Extractor) WithUsageHook(hook func(provider.Usage)) *Extractor {
	e.onUsage = hook
	return e
}

// WithNoteMaxAgeDays limits dedup/merge/update to notes created within the given number of days.
// 0 or negative means no limit (all notes are eligible).
func (e *Extractor) WithNoteMaxAgeDays(days int) *Extractor {
	e.noteMaxAgeDays = days
	return e
}

// ExtractTurn analyses a single user→assistant exchange and persists any notes.
func (e *Extractor) ExtractTurn(ctx context.Context, profileID, profileSlug, conversationID string, sourceMessageIDs []string, userMsg, assistantMsg string) (*ExtractionResult, error) {
	if e.adapter == nil {
		return &ExtractionResult{}, nil
	}

	var existing []*store.MemoryNote
	if notes, err := e.noteRepo.ListByProfile(ctx, profileID); err == nil {
		existing = notes
	}
	resp := e.llmExtract(ctx, profileSlug, userMsg, assistantMsg, existing)
	if !resp.HasNewInfo || resp.Confidence < 0.6 || len(resp.Notes) == 0 {
		var reason string
		switch {
		case !resp.HasNewInfo:
			reason = "has_new_info=false"
		case resp.Confidence < 0.6:
			reason = fmt.Sprintf("confidence too low: %.2f < 0.6", resp.Confidence)
		default:
			reason = "notes array empty"
		}
		e.logHook.ExtractionPayloadRejected(reason)
		return &ExtractionResult{}, nil
	}
	items := resp.Notes
	reconcileAdditions(existing, items, e.noteMaxAgeDays)
	updatedNotes := make([]*store.MemoryNote, 0)
	addItems := make([]extractedItem, 0, len(items))
	updatedCount := 0
	mergedCount := 0
	for _, it := range items {
		if it.Action == "update" && it.TargetID != "" {
			if note, updated, err := e.updateNote(ctx, profileID, it.TargetID, []extractedItem{it}, sourceMessageIDs); err == nil && updated {
				updatedNotes = append(updatedNotes, note)
				updatedCount++
				continue
			}
		}
		if it.Action == "merge" && len(it.mergeIDs) > 0 {
			for _, id := range it.mergeIDs {
				_ = e.noteRepo.Delete(ctx, id)
			}
			it.Action = "add"
			it.TargetID = ""
			it.mergeIDs = nil
			addItems = append(addItems, it)
			mergedCount++
			continue
		}
		addItems = append(addItems, it)
	}

	processor := NewProcessor(e.noteRepo, e.deduper, e.logHook)
	if e.noteRepo != nil && e.noteRepo.DB() != nil {
		processor.WithSummaryRollups(NewSummaryRollupBuilder(e.noteRepo, store.NewMemorySummaryRepo(e.noteRepo.DB())))
	}
	notes, updated, err := processor.Process(ctx, profileID, conversationID, addItems, sourceMessageIDs)
	if err != nil {
		return nil, err
	}

	if (len(notes) > 0 || len(updatedNotes) > 0) && e.invalidator != nil {
		_ = e.invalidator.InvalidateAll(ctx, profileID)
	}

	return &ExtractionResult{Notes: notes, UpdatedNotes: updatedNotes, Updated: updated + updatedCount, Merged: mergedCount}, nil
}

// llmExtract calls the model and parses the JSON response. Never returns an error
// — failures are silently dropped so a bad extraction never breaks the chat flow.
func (e *Extractor) llmExtract(ctx context.Context, profileSlug, userMsg, assistantMsg string, existing []*store.MemoryNote) extractionResponse {
	template, _, err := config.ReadExtractorPromptFile(profileSlug)
	if err != nil || strings.TrimSpace(template) == "" {
		template = config.DefaultExtractorPromptTemplate
	}
	prompt := fmt.Sprintf(template, formatExistingNotes(existing), userMsg, assistantMsg)
	resp, err := e.adapter.Complete(ctx, provider.CompletionRequest{
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: 0.2,
	})
	if err != nil {
		return extractionResponse{}
	}
	if e.onUsage != nil {
		e.onUsage(resp.Usage)
	}

	raw := strings.TrimSpace(resp.Content)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var payload extractionResponse
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		e.logHook.ExtractionPayloadRejected("invalid json")
		return extractionResponse{}
	}
	if err := validateExtractionPayload(payload); err != nil {
		e.logHook.ExtractionPayloadRejected(err.Error())
		return extractionResponse{}
	}
	return payload
}

func validateExtractionPayload(payload extractionResponse) error {
	if payload.Confidence < 0 || payload.Confidence > 1 {
		return rejectErr("confidence out of range")
	}
	if !payload.HasNewInfo && len(payload.Notes) > 0 {
		return rejectErr("has_new_info=false with non-empty notes")
	}
	if payload.HasNewInfo && len(payload.Notes) == 0 {
		return rejectErr("has_new_info=true with empty notes")
	}
	for i, n := range payload.Notes {
		if n.Action != "add" && n.Action != "update" && n.Action != "merge" {
			return rejectErr("note[%d]: invalid action", i)
		}
		if n.Action == "update" && strings.TrimSpace(n.TargetID) == "" {
			return rejectErr("note[%d]: update requires target_id", i)
		}
		switch n.Category {
		case "fact", "progress", "blocker", "action_item", "other":
		default:
			return rejectErr("note[%d]: invalid category", i)
		}
		if strings.TrimSpace(n.Content) == "" {
			return rejectErr("note[%d]: empty content", i)
		}
	}
	return nil
}

func formatExistingNotes(existing []*store.MemoryNote) string {
	if len(existing) == 0 {
		return "(none)"
	}
	if len(existing) > 25 {
		existing = existing[:25]
	}
	lines := make([]string, 0, len(existing))
	for i, n := range existing {
		lines = append(lines, fmt.Sprintf("- [#%d] (%s) %s  → target_id: %s", i+1, n.Category, n.Content, n.ID))
	}
	return strings.Join(lines, "\n")
}

func (e *Extractor) updateNote(ctx context.Context, profileID, targetID string, items []extractedItem, sourceMessageIDs []string) (*store.MemoryNote, bool, error) {
	if len(items) == 0 {
		return nil, false, nil
	}
	note, err := e.noteRepo.GetByID(ctx, targetID)
	if err != nil {
		return nil, false, err
	}
	if note.ProfileID != profileID {
		return nil, false, nil
	}
	item := items[0]
	if strings.TrimSpace(item.Content) == "" {
		return nil, false, nil
	}

	note.Content = item.Content
	note.Importance = item.Importance
	cat := store.MemoryCategory(item.Category)
	switch cat {
	case store.CategoryFact, store.CategoryProgress, store.CategoryBlocker, store.CategoryActionItem, store.CategoryOther:
		note.Category = cat
	default:
		note.Category = store.CategoryOther
	}

	if sourceMessageIDs != nil {
		if merged, changed, err := mergeSourceMessageIDs(note.SourceMessageIDs, sourceMessageIDs); err == nil && changed {
			note.SourceMessageIDs = merged
		}
	}
	if err := e.noteRepo.Update(ctx, note); err != nil {
		return nil, false, err
	}
	if e.noteRepo != nil && e.noteRepo.DB() != nil {
		_ = NewSummaryRollupBuilder(e.noteRepo, store.NewMemorySummaryRepo(e.noteRepo.DB())).MarkCoveredSummariesStale(ctx, profileID, note.CreatedAt)
	}
	if e.invalidator != nil {
		_ = e.invalidator.InvalidateAll(ctx, profileID)
	}
	return note, true, nil
}

// reconcileAdditions checks "add" items against existing notes shown to the LLM.
// If an "add" item has high word-overlap with existing notes within the age window,
// it's auto-converted to "update" (1 match) or "merge" (2+ matches). This is a
// safety net for when the LLM outputs "add" instead of "update"/"merge".
// maxAgeDays <= 0 means no age limit.
func reconcileAdditions(existing []*store.MemoryNote, items []extractedItem, maxAgeDays int) {
	if len(existing) == 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
	for i, item := range items {
		if item.Action != "add" {
			continue
		}
		itemWords := tokenize(item.Content)
		if len(itemWords) == 0 {
			continue
		}

		var matchedIDs []string
		var bestID string
		var bestScore float64

		for _, note := range existing {
			if maxAgeDays > 0 && note.CreatedAt.Before(cutoff) {
				continue
			}
			noteWords := tokenize(note.Content)
			if len(noteWords) == 0 {
				continue
			}
			score := jaccardSimilarity(itemWords, noteWords)
			if score >= textDedupThreshold {
				matchedIDs = append(matchedIDs, note.ID)
			}
			if score > bestScore {
				bestScore = score
				bestID = note.ID
			}
		}

		if bestID != "" && bestScore >= textDedupThreshold {
			if len(matchedIDs) > 1 {
				items[i].Action = "merge"
				items[i].mergeIDs = matchedIDs
			} else {
				items[i].Action = "update"
				items[i].TargetID = bestID
			}
		}
	}
}

func tokenize(s string) []string {
	words := strings.Fields(strings.ToLower(s))
	out := make([]string, 0, len(words))
	for _, w := range words {
		out = append(out, strings.Trim(w, ".,!?;:\"'()[]{}"))
	}
	return out
}

func jaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[string]struct{}, len(a))
	for _, w := range a {
		setA[w] = struct{}{}
	}
	setB := make(map[string]struct{}, len(b))
	for _, w := range b {
		setB[w] = struct{}{}
	}
	inter := 0
	for w := range setA {
		if _, ok := setB[w]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

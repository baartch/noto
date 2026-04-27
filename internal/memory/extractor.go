package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"noto/internal/config"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/vector"
)

// ExtractionResult holds the notes extracted from a single exchange.
type ExtractionResult struct {
	Notes        []*store.MemoryNote
	UpdatedNotes []*store.MemoryNote
	Updated      int
}

// extractionResponse is the JSON shape the LLM returns for an extraction.
type extractionResponse struct {
	HasNewInfo bool            `json:"has_new_info"`
	Confidence float64         `json:"confidence"`
	Notes      []extractedItem `json:"notes"`
}

// extractedItem is the JSON shape the LLM returns per note.
type extractedItem struct {
	Action     string `json:"action"`    // add | update
	TargetID   string `json:"target_id"` // required when action=update
	Category   string `json:"category"`  // fact | progress | blocker | action_item | other
	Content    string `json:"content"`
	Importance int    `json:"importance"` // 1-10
}


// CacheInvalidator invalidates cached memory retrieval context.
type CacheInvalidator interface {
	InvalidateAll(ctx context.Context, profileID string) error
}

// Extractor extracts memory notes using the LLM and persists them to SQLite.
type Extractor struct {
	noteRepo    *store.MemoryNoteRepo
	adapter     provider.Adapter // nil disables extraction
	invalidator CacheInvalidator
	deduper     vector.Deduper
	logHook     CaptureLogHook
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

// ExtractTurn analyses a single user→assistant exchange and persists any notes.
func (e *Extractor) ExtractTurn(ctx context.Context, profileID, conversationID string, sourceMessageIDs []string, userMsg, assistantMsg string) (*ExtractionResult, error) {
	if e.adapter == nil {
		return &ExtractionResult{}, nil
	}

	var existing []*store.MemoryNote
	if notes, err := e.noteRepo.ListByProfile(ctx, profileID); err == nil {
		existing = notes
	}
	resp := e.llmExtract(ctx, profileID, userMsg, assistantMsg, existing)
	if !resp.HasNewInfo || resp.Confidence < 0.6 || len(resp.Notes) == 0 {
		return &ExtractionResult{}, nil
	}
	items := resp.Notes
	updatedNotes := make([]*store.MemoryNote, 0)
	addItems := make([]extractedItem, 0, len(items))
	updatedCount := 0
	for _, it := range items {
		if it.Action == "update" && it.TargetID != "" {
			if note, updated, err := e.updateNote(ctx, profileID, it.TargetID, []extractedItem{it}, sourceMessageIDs); err == nil && updated {
				updatedNotes = append(updatedNotes, note)
				updatedCount++
				continue
			}
		}
		addItems = append(addItems, it)
	}

	processor := NewProcessor(e.noteRepo, e.deduper, e.logHook)
	notes, updated, err := processor.Process(ctx, profileID, conversationID, addItems, sourceMessageIDs)
	if err != nil {
		return nil, err
	}

	if (len(notes) > 0 || len(updatedNotes) > 0) && e.invalidator != nil {
		_ = e.invalidator.InvalidateAll(ctx, profileID)
	}

	return &ExtractionResult{Notes: notes, UpdatedNotes: updatedNotes, Updated: updated + updatedCount}, nil
}

// llmExtract calls the model and parses the JSON response. Never returns an error
// — failures are silently dropped so a bad extraction never breaks the chat flow.
func (e *Extractor) llmExtract(ctx context.Context, profileID, userMsg, assistantMsg string, existing []*store.MemoryNote) extractionResponse {
	template, _, err := config.ReadExtractorPromptFile(profileID)
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
		if n.Action != "add" && n.Action != "update" {
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
	if len(existing) > 50 {
		existing = existing[:50]
	}
	lines := make([]string, 0, len(existing))
	for _, n := range existing {
		lines = append(lines, fmt.Sprintf("- %s | (%s) %s", n.ID, n.Category, n.Content))
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
	if e.invalidator != nil {
		_ = e.invalidator.InvalidateAll(ctx, profileID)
	}
	return note, true, nil
}

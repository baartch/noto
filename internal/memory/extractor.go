package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"noto/internal/provider"
	"noto/internal/store"
)

// ExtractionResult holds the notes extracted from a single exchange.
type ExtractionResult struct {
	Notes   []*store.MemoryNote
	Updated int
}

// extractionResponse is the JSON shape the LLM returns for an extraction.
type extractionResponse struct {
	HasNewInfo bool            `json:"has_new_info"`
	Confidence float64         `json:"confidence"`
	Notes      []extractedItem `json:"notes"`
	Action     string          `json:"action"`    // add | update
	TargetID   string          `json:"target_id"` // note id when action=update
}

// extractedItem is the JSON shape the LLM returns per note.
type extractedItem struct {
	Category   string `json:"category"` // fact | progress | blocker | action_item | other
	Content    string `json:"content"`
	Importance int    `json:"importance"` // 1-10
}

type dedupeResult struct {
	IsNew  bool   `json:"is_new"`
	Reason string `json:"reason"`
}

const extractionPrompt = `Extract memory-worthy facts from this conversation exchange.
Reply ONLY with JSON (no markdown, no explanation). Language: match the user's language.

Return shape:
{"has_new_info": true|false, "confidence": 0.0-1.0, "action": "add|update", "target_id": "", "notes": [
  {"category":"fact|progress|blocker|action_item|other","content":"one concise sentence, max 150 chars","importance":1-10}
]}

Rules:
- If nothing is worth remembering, set "has_new_info": false, "confidence": 0, and "notes": []
- If the user clarifies/corrects something already captured, set action="update" and pick a target_id from the existing notes list
- Otherwise use action="add" and leave target_id empty
- importance 8-10: critical facts about the user (name, role, key goals, decisions)
- importance 5-7: useful context (preferences, current work, recent events)
- importance 1-4: minor details

Existing notes (pick target_id from here if updating):
%s

Exchange:
User: %s
Assistant: %s`

const dedupePrompt = `You are a memory deduplication assistant. Given a candidate note and a list of existing notes, decide if the candidate is NEW.
Reply ONLY with JSON: {"is_new": true|false, "reason": "short reason"}

Consider paraphrases as duplicates (same meaning, different wording). Only mark true if the candidate adds new information.

Existing notes:
%s

Candidate note:
%s`

// CacheInvalidator invalidates cached memory retrieval context.
type CacheInvalidator interface {
	InvalidateAll(ctx context.Context, profileID string) error
}

// Extractor extracts memory notes using the LLM and persists them to SQLite.
type Extractor struct {
	noteRepo    *store.MemoryNoteRepo
	adapter     provider.Adapter // nil disables extraction
	invalidator CacheInvalidator
}

// NewExtractor creates an Extractor. Pass nil adapter to disable LLM extraction.
func NewExtractor(noteRepo *store.MemoryNoteRepo, adapter provider.Adapter, invalidator CacheInvalidator) *Extractor {
	return &Extractor{noteRepo: noteRepo, adapter: adapter, invalidator: invalidator}
}

// ExtractTurn analyses a single user→assistant exchange and persists any notes.
func (e *Extractor) ExtractTurn(ctx context.Context, profileID, conversationID, userMsg, assistantMsg string) (*ExtractionResult, error) {
	if e.adapter == nil {
		return &ExtractionResult{}, nil
	}

	var existing []*store.MemoryNote
	if notes, err := e.noteRepo.ListByProfile(ctx, profileID); err == nil {
		existing = notes
	}
	resp := e.llmExtract(ctx, userMsg, assistantMsg, existing)
	if !resp.HasNewInfo || resp.Confidence < 0.6 || len(resp.Notes) == 0 {
		return &ExtractionResult{}, nil
	}
	items := resp.Notes

	if resp.Action == "update" && resp.TargetID != "" {
		if updated, err := e.updateNote(ctx, profileID, resp.TargetID, items); err == nil && updated {
			return &ExtractionResult{Notes: []*store.MemoryNote{}, Updated: 1}, nil
		}
	}

	// Semantic de-duplication: filter out notes that are already known.
	filtered := items
	if len(items) > 0 && len(existing) > 0 {
		filtered = e.filterNewNotes(ctx, items, existing)
	}
	if len(filtered) == 0 {
		return &ExtractionResult{}, nil
	}

	var notes []*store.MemoryNote
	for _, item := range filtered {
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		if item.Importance < 1 {
			item.Importance = 5
		}
		cat := store.MemoryCategory(item.Category)
		switch cat {
		case store.CategoryFact, store.CategoryProgress,
			store.CategoryBlocker, store.CategoryActionItem, store.CategoryOther:
		default:
			cat = store.CategoryOther
		}
		note := &store.MemoryNote{
			ID:               fmt.Sprintf("mn-%x", time.Now().UnixNano()),
			ProfileID:        profileID,
			ConversationID:   conversationID,
			Category:         cat,
			Content:          item.Content,
			Importance:       item.Importance,
			SourceMessageIDs: "[]",
		}
		if err := e.noteRepo.Create(ctx, note); err != nil {
			return nil, fmt.Errorf("memory: save note: %w", err)
		}
		notes = append(notes, note)
	}

	if len(notes) > 0 && e.invalidator != nil {
		_ = e.invalidator.InvalidateAll(ctx, profileID)
	}

	return &ExtractionResult{Notes: notes}, nil
}

// llmExtract calls the model and parses the JSON response. Never returns an error
// — failures are silently dropped so a bad extraction never breaks the chat flow.
func (e *Extractor) llmExtract(ctx context.Context, userMsg, assistantMsg string, existing []*store.MemoryNote) extractionResponse {
	prompt := fmt.Sprintf(extractionPrompt, formatExistingNotes(existing), userMsg, assistantMsg)
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
		return extractionResponse{}
	}
	return payload
}

// filterNewNotes uses an LLM-based semantic check to remove duplicates.
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

func (e *Extractor) filterNewNotes(ctx context.Context, items []extractedItem, existing []*store.MemoryNote) []extractedItem {
	// Limit to last 50 notes to keep prompt bounded.
	if len(existing) > 50 {
		existing = existing[len(existing)-50:]
	}

	var existingLines []string
	for _, n := range existing {
		existingLines = append(existingLines, fmt.Sprintf("- (%s) %s", n.Category, n.Content))
	}
	existingBlock := strings.Join(existingLines, "\n")
	if existingBlock == "" {
		return items
	}

	var out []extractedItem
	for _, item := range items {
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		prompt := fmt.Sprintf(dedupePrompt, existingBlock, item.Content)
		resp, err := e.adapter.Complete(ctx, provider.CompletionRequest{
			Messages:    []provider.Message{{Role: "user", Content: prompt}},
			Temperature: 0.0,
		})
		if err != nil {
			// If dedupe fails, keep the note (fail-open) so we don't lose data.
			out = append(out, item)
			continue
		}
		raw := strings.TrimSpace(resp.Content)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)

		var result dedupeResult
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			out = append(out, item)
			continue
		}
		if result.IsNew {
			out = append(out, item)
		}
	}
	return out
}

func (e *Extractor) updateNote(ctx context.Context, profileID, targetID string, items []extractedItem) (bool, error) {
	if len(items) == 0 {
		return false, nil
	}
	note, err := e.noteRepo.GetByID(ctx, targetID)
	if err != nil {
		return false, err
	}
	if note.ProfileID != profileID {
		return false, nil
	}
	item := items[0]
	if strings.TrimSpace(item.Content) == "" {
		return false, nil
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

	if err := e.noteRepo.Update(ctx, note); err != nil {
		return false, err
	}
	if e.invalidator != nil {
		_ = e.invalidator.InvalidateAll(ctx, profileID)
	}
	return true, nil
}

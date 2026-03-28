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
	Notes []*store.MemoryNote
}

// extractionResponse is the JSON shape the LLM returns for an extraction.
type extractionResponse struct {
	HasNewInfo bool           `json:"has_new_info"`
	Confidence float64        `json:"confidence"`
	Notes      []extractedItem `json:"notes"`
}

// extractedItem is the JSON shape the LLM returns per note.
type extractedItem struct {
	Category   string `json:"category"`   // fact | progress | blocker | action_item | other
	Content    string `json:"content"`
	Importance int    `json:"importance"` // 1-10
}

type dedupeResult struct {
	IsNew  bool   `json:"is_new"`
	Reason string `json:"reason"`
}

const extractionPrompt = `Extract memory-worthy facts from this conversation exchange.
Reply ONLY with JSON (no markdown, no explanation). Language: match the conversation language.

Return shape:
{"has_new_info": true|false, "confidence": 0.0-1.0, "notes": [
  {"category":"fact|progress|blocker|action_item|other","content":"one concise sentence, max 150 chars","importance":1-10}
]}

Rules:
- If nothing is worth remembering, set "has_new_info": false, "confidence": 0, and "notes": []
- importance 8-10: critical facts about the user (name, role, key goals, decisions)
- importance 5-7: useful context (preferences, current work, recent events)
- importance 1-4: minor details

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

// Extractor extracts memory notes using the LLM and persists them to SQLite.
type CacheInvalidator interface {
	InvalidateAll(ctx context.Context, profileID string) error
}

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

	resp := e.llmExtract(ctx, userMsg, assistantMsg)
	if !resp.HasNewInfo || resp.Confidence < 0.6 || len(resp.Notes) == 0 {
		return &ExtractionResult{}, nil
	}
	items := resp.Notes

	// Semantic de-duplication: filter out notes that are already known.
	filtered := items
	if len(items) > 0 {
		existing, err := e.noteRepo.ListByProfile(ctx, profileID)
		if err == nil {
			filtered = e.filterNewNotes(ctx, items, existing)
		}
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
func (e *Extractor) llmExtract(ctx context.Context, userMsg, assistantMsg string) extractionResponse {
	prompt := fmt.Sprintf(extractionPrompt, userMsg, assistantMsg)
	resp, err := e.adapter.Complete(ctx, provider.CompletionRequest{
		Messages: []provider.Message{{Role: "user", Content: prompt}},
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
			Messages: []provider.Message{{Role: "user", Content: prompt}},
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

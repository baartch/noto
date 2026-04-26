package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/vector"
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

const extractionPrompt = `You are a memory extractor for a chat assistant.
Return ONLY valid JSON. No markdown. No code fences. No commentary.
Language policy: write note content in the same language as the user message.

Output schema (all keys required):
{
  "has_new_info": true|false,
  "confidence": 0.0-1.0,
  "action": "add|update",
  "target_id": "",
  "notes": [
    {
      "category": "fact|progress|blocker|action_item|other",
      "content": "max 220 chars, one concise sentence",
      "importance": 1-10
    }
  ]
}

Hard rules:
1) Always emit strict JSON (double quotes, no trailing commas).
2) Prioritize USER-provided information over assistant text.
   - Extract primarily from the user message.
   - Use assistant text only as context/confirmation, not as a source of new facts.
   - If user and assistant conflict, trust the user.
3) If nothing memory-worthy exists:
   - "has_new_info": false
   - "confidence": 0
   - "action": "add"
   - "target_id": ""
   - "notes": []
4) Use "action":"update" ONLY when the user clearly corrects/refines existing memory.
   - Then set "target_id" to an ID from Existing notes.
   - Include exactly one replacement note in "notes".
5) For "action":"add", set "target_id":"".
6) Do not duplicate existing notes; prefer update when correcting, add when new.
7) Keep note content atomic and specific (no lists, no combined topics).

Importance rubric:
- 8-10: durable identity/long-term goals/major decisions likely useful across future sessions.
- 5-7: medium-term preferences, ongoing work context, recent important events.
- 1-4: minor or short-lived details.

Existing notes (use for update targeting):
%s

Exchange:
User: %s
Assistant: %s`

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
	resp := e.llmExtract(ctx, userMsg, assistantMsg, existing)
	if !resp.HasNewInfo || resp.Confidence < 0.6 || len(resp.Notes) == 0 {
		return &ExtractionResult{}, nil
	}
	items := resp.Notes

	if resp.Action == "update" && resp.TargetID != "" {
		if updated, err := e.updateNote(ctx, profileID, resp.TargetID, items, sourceMessageIDs); err == nil && updated {
			return &ExtractionResult{Notes: []*store.MemoryNote{}, Updated: 1}, nil
		}
	}

	processor := NewProcessor(e.noteRepo, e.deduper, e.logHook)
	notes, updated, err := processor.Process(ctx, profileID, conversationID, items, sourceMessageIDs)
	if err != nil {
		return nil, err
	}

	if len(notes) > 0 && e.invalidator != nil {
		_ = e.invalidator.InvalidateAll(ctx, profileID)
	}

	return &ExtractionResult{Notes: notes, Updated: updated}, nil
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

func (e *Extractor) updateNote(ctx context.Context, profileID, targetID string, items []extractedItem, sourceMessageIDs []string) (bool, error) {
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

	if sourceMessageIDs != nil {
		if merged, changed, err := mergeSourceMessageIDs(note.SourceMessageIDs, sourceMessageIDs); err == nil && changed {
			note.SourceMessageIDs = merged
		}
	}
	if err := e.noteRepo.Update(ctx, note); err != nil {
		return false, err
	}
	if e.invalidator != nil {
		_ = e.invalidator.InvalidateAll(ctx, profileID)
	}
	return true, nil
}

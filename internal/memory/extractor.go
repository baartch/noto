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

// extractedItem is the JSON shape the LLM returns per note.
type extractedItem struct {
	Category   string `json:"category"`   // fact | progress | blocker | action_item | other
	Content    string `json:"content"`
	Importance int    `json:"importance"` // 1-10
}

const extractionPrompt = `Extract memory-worthy facts from this conversation exchange.
Reply ONLY with a JSON array (no markdown, no explanation). Language: match the conversation language.

Each object: {"category":"fact|progress|blocker|action_item|other","content":"one concise sentence, max 150 chars","importance":1-10}

Rules:
- importance 8-10: critical facts about the user (name, role, key goals, decisions)
- importance 5-7: useful context (preferences, current work, recent events)
- importance 1-4: minor details
- Return [] if nothing is worth remembering long-term

Exchange:
User: %s
Assistant: %s`

// Extractor extracts memory notes using the LLM and persists them to SQLite.
type Extractor struct {
	noteRepo *store.MemoryNoteRepo
	adapter  provider.Adapter // nil = fall back to heuristic only
}

// NewExtractor creates an Extractor. Pass nil adapter to disable LLM extraction.
func NewExtractor(noteRepo *store.MemoryNoteRepo, adapter provider.Adapter) *Extractor {
	return &Extractor{noteRepo: noteRepo, adapter: adapter}
}

// ExtractTurn analyses a single user→assistant exchange and persists any notes.
func (e *Extractor) ExtractTurn(ctx context.Context, profileID, conversationID, userMsg, assistantMsg string) (*ExtractionResult, error) {
	var items []extractedItem

	if e.adapter != nil {
		items = e.llmExtract(ctx, userMsg, assistantMsg)
	}

	// Nothing from LLM (or no adapter) — nothing to save.
	if len(items) == 0 {
		return &ExtractionResult{}, nil
	}

	var notes []*store.MemoryNote
	for _, item := range items {
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

	return &ExtractionResult{Notes: notes}, nil
}

// llmExtract calls the model and parses the JSON response. Never returns an error
// — failures are silently dropped so a bad extraction never breaks the chat flow.
func (e *Extractor) llmExtract(ctx context.Context, userMsg, assistantMsg string) []extractedItem {
	prompt := fmt.Sprintf(extractionPrompt, userMsg, assistantMsg)
	resp, err := e.adapter.Complete(ctx, provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2, // low temperature for consistent structured output
	})
	if err != nil {
		return nil
	}

	// Strip markdown code fences if the model wrapped the JSON.
	raw := strings.TrimSpace(resp.Content)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var items []extractedItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

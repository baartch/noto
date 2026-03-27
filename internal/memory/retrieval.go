package memory

import (
	"context"
	"fmt"
	"strings"

	"noto/internal/store"
)

// RetrievalContext is the assembled context payload for a chat turn.
type RetrievalContext struct {
	// SystemPrompt is the profile's system prompt.
	SystemPrompt string

	// MemoryBlock is the formatted block of relevant memory notes.
	MemoryBlock string

	// SessionSummary is the most recent session summary text.
	SessionSummary string

	// AssembledPrompt is the final combined system prompt with injected context.
	AssembledPrompt string
}

// Retrieval assembles context for a chat turn from SQLite source-of-truth data.
type Retrieval struct {
	noteRepo    *store.MemoryNoteRepo
	summaryRepo *store.SessionSummaryRepo
}

// NewRetrieval creates a Retrieval service.
func NewRetrieval(noteRepo *store.MemoryNoteRepo, summaryRepo *store.SessionSummaryRepo) *Retrieval {
	return &Retrieval{noteRepo: noteRepo, summaryRepo: summaryRepo}
}

// Assemble builds the RetrievalContext for a profile, reading from SQLite.
func (r *Retrieval) Assemble(ctx context.Context, profileID, systemPrompt string) (*RetrievalContext, error) {
	notes, err := r.noteRepo.ListByProfile(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("memory: list notes: %w", err)
	}

	summary, err := r.summaryRepo.GetLatestByProfile(ctx, profileID)
	var summaryText string
	if err == nil {
		summaryText = summary.SummaryText
	}

	memoryBlock := buildMemoryBlock(notes)

	assembled := buildAssembledPrompt(systemPrompt, summaryText, memoryBlock)

	return &RetrievalContext{
		SystemPrompt:    systemPrompt,
		MemoryBlock:     memoryBlock,
		SessionSummary:  summaryText,
		AssembledPrompt: assembled,
	}, nil
}

func buildMemoryBlock(notes []*store.MemoryNote) string {
	if len(notes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Memory Notes\n")
	for _, n := range notes {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", n.Category, n.Content))
	}
	return sb.String()
}

func buildAssembledPrompt(systemPrompt, sessionSummary, memoryBlock string) string {
	parts := []string{systemPrompt}
	if sessionSummary != "" {
		parts = append(parts, "\n## Previous Session Summary\n"+sessionSummary)
	}
	if memoryBlock != "" {
		parts = append(parts, "\n"+memoryBlock)
	}
	return strings.Join(parts, "\n")
}

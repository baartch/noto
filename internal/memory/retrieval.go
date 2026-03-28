package memory

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

	// CacheHit indicates the assembled prompt was served from cache.
	CacheHit bool
}

// Retrieval assembles context for a chat turn from SQLite source-of-truth data.
type CacheRepository interface {
	Get(ctx context.Context, profileID, cacheKey string) (*store.ContextCacheEntry, error)
	Upsert(ctx context.Context, e *store.ContextCacheEntry) error
	Invalidate(ctx context.Context, profileID, cacheKey string) error
}

type Retrieval struct {
	noteRepo    *store.MemoryNoteRepo
	summaryRepo *store.SessionSummaryRepo
	cacheRepo   CacheRepository
}

// NewRetrieval creates a Retrieval service.
func NewRetrieval(noteRepo *store.MemoryNoteRepo, summaryRepo *store.SessionSummaryRepo, cacheRepo CacheRepository) *Retrieval {
	return &Retrieval{noteRepo: noteRepo, summaryRepo: summaryRepo, cacheRepo: cacheRepo}
}

// Assemble builds the RetrievalContext for a profile, reading from SQLite.
// It reuses cached context if available and valid.
func (r *Retrieval) Assemble(ctx context.Context, profileID, systemPrompt string) (*RetrievalContext, error) {
	summaryText := ""
	summaryID := "none"
	if r.summaryRepo != nil {
		summary, err := r.summaryRepo.GetLatestByProfile(ctx, profileID)
		if err == nil {
			summaryText = summary.SummaryText
			summaryID = summary.ID
		}
	}

	cacheKey := cacheKeyFor(profileID, systemPrompt, summaryID)

	if r.cacheRepo != nil {
		cached, err := r.cacheRepo.Get(ctx, profileID, cacheKey)
		if err == nil && cached != nil {
			var cachedCtx RetrievalContext
			if err := json.Unmarshal([]byte(cached.Payload), &cachedCtx); err == nil {
				cachedCtx.CacheHit = true
				return &cachedCtx, nil
			}
			_ = r.cacheRepo.Invalidate(ctx, profileID, cacheKey)
		}
	}

	notes, err := r.noteRepo.ListByProfile(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("memory: list notes: %w", err)
	}

	memoryBlock := buildMemoryBlock(notes)

	assembled := buildAssembledPrompt(systemPrompt, summaryText, memoryBlock)

	ctxOut := &RetrievalContext{
		SystemPrompt:    systemPrompt,
		MemoryBlock:     memoryBlock,
		SessionSummary:  summaryText,
		AssembledPrompt: assembled,
		CacheHit:        false,
	}

	if r.cacheRepo != nil {
		payload, _ := json.Marshal(ctxOut)
		expires := time.Now().Add(24 * time.Hour)
		_ = r.cacheRepo.Upsert(ctx, &store.ContextCacheEntry{
			ID:        fmt.Sprintf("cc-%x", time.Now().UnixNano()),
			ProfileID: profileID,
			CacheKey:  cacheKey,
			Payload:   string(payload),
			CreatedAt: time.Now().UTC(),
			ExpiresAt: &expires,
		})
	}

	return ctxOut, nil
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

func cacheKeyFor(profileID, systemPrompt, summaryID string) string {
	hash := sha256.Sum256([]byte(profileID + "::" + systemPrompt + "::" + summaryID))
	return fmt.Sprintf("ctx:%x", hash)
}


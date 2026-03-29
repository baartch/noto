package memory

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"noto/internal/store"
	"noto/internal/vector"
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

// CacheRepository manages cached assembled prompt payloads.
type CacheRepository interface {
	Get(ctx context.Context, profileID, cacheKey string) (*store.ContextCacheEntry, error)
	Upsert(ctx context.Context, e *store.ContextCacheEntry) error
	Invalidate(ctx context.Context, profileID, cacheKey string) error
}

// Retrieval assembles context for a chat turn from SQLite source-of-truth data.
type Retrieval struct {
	noteRepo        *store.MemoryNoteRepo
	summaryRepo     *store.SessionSummaryRepo
	cacheRepo       CacheRepository
	vectorIndexPath string
	warnFn          func(error)
}

// RetrievalOption configures Retrieval behavior.
type RetrievalOption func(*Retrieval)

// WithVectorIndexPath sets the vector index path for warning checks.
func WithVectorIndexPath(path string) RetrievalOption {
	return func(r *Retrieval) {
		r.vectorIndexPath = path
	}
}

// WithWarnFunc registers a warning hook for vector index issues.
func WithWarnFunc(fn func(error)) RetrievalOption {
	return func(r *Retrieval) {
		r.warnFn = fn
	}
}

// NewRetrieval creates a Retrieval service.
func NewRetrieval(noteRepo *store.MemoryNoteRepo, summaryRepo *store.SessionSummaryRepo, cacheRepo CacheRepository, opts ...RetrievalOption) *Retrieval {
	r := &Retrieval{noteRepo: noteRepo, summaryRepo: summaryRepo, cacheRepo: cacheRepo}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Assemble builds the RetrievalContext for a profile, reading from SQLite.
// It reuses cached context if available and valid.
func (r *Retrieval) Assemble(ctx context.Context, profileID, systemPrompt string) (*RetrievalContext, error) {
	if err := r.checkVectorIndex(); err != nil && r.warnFn != nil {
		r.warnFn(err)
	}
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

	memoryBlock := BuildMemoryBlock(notes)

	assembled := AssemblePrompt(systemPrompt, summaryText, memoryBlock)

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

// BuildMemoryBlock formats notes into the memory block for prompts.
func BuildMemoryBlock(notes []*store.MemoryNote) string {
	if len(notes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Memory Notes\n")
	for _, n := range notes {
		fmt.Fprintf(&sb, "- [%s] %s\n", n.Category, n.Content)
	}
	return sb.String()
}

// AssemblePrompt merges system prompt, summary, and memory block into the final prompt.
func AssemblePrompt(systemPrompt, sessionSummary, memoryBlock string) string {
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

func (r *Retrieval) checkVectorIndex() error {
	if r.vectorIndexPath == "" {
		return nil
	}
	info, err := os.Stat(r.vectorIndexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return vector.ErrIndexNotFound
		}
		return vector.ErrIndexCorrupted
	}
	if info.Size() == 0 {
		return vector.ErrIndexCorrupted
	}
	return nil
}

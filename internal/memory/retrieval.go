package memory

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"noto/internal/provider"
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

const vectorTopK = 6

// Retrieval assembles context for a chat turn from SQLite source-of-truth data.
type Retrieval struct {
	noteRepo        *store.MemoryNoteRepo
	summaryRepo     *store.SessionSummaryRepo
	cacheRepo       CacheRepository
	vectorIndexPath string
	warnFn          func(error)
	tokenBudget     int
	vectorIndex     vector.Index
	profileID       string
	embedder        vector.Embedder
	embeddingModel  string
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

// WithTokenBudget sets the token budget for selecting memory notes.
func WithTokenBudget(budget int) RetrievalOption {
	return func(r *Retrieval) {
		r.tokenBudget = budget
	}
}

// WithVectorRetrieval wires vector ranking into Retrieval.
func WithVectorRetrieval(index vector.Index, profileID string, embedder vector.Embedder, model string) RetrievalOption {
	return func(r *Retrieval) {
		r.vectorIndex = index
		r.profileID = profileID
		r.embedder = embedder
		r.embeddingModel = model
	}
}

// NewRetrieval creates a Retrieval service.
func NewRetrieval(noteRepo *store.MemoryNoteRepo, summaryRepo *store.SessionSummaryRepo, cacheRepo CacheRepository, opts ...RetrievalOption) *Retrieval {
	r := &Retrieval{noteRepo: noteRepo, summaryRepo: summaryRepo, cacheRepo: cacheRepo}
	for _, opt := range opts {
		opt(r)
	}
	if r.tokenBudget <= 0 {
		r.tokenBudget = 1500
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

	cacheKey := cacheKeyFor(profileID, systemPrompt, summaryID, r.tokenBudget)

	if r.cacheRepo != nil {
		cached, err := r.cacheRepo.Get(ctx, profileID, cacheKey)
		if err == nil && cached != nil {
			if cached.ExpiresAt != nil && cached.ExpiresAt.Before(time.Now()) {
				_ = r.cacheRepo.Invalidate(ctx, profileID, cacheKey)
			} else {
				var cachedCtx RetrievalContext
				if err := json.Unmarshal([]byte(cached.Payload), &cachedCtx); err == nil {
					cachedCtx.CacheHit = true
					return &cachedCtx, nil
				}
				_ = r.cacheRepo.Invalidate(ctx, profileID, cacheKey)
			}
		}
	}

	notes, err := r.noteRepo.ListByProfile(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("memory: list notes: %w", err)
	}

	rankedIDs, err := r.rankNotes(ctx, systemPrompt, summaryText)
	if err != nil && r.warnFn != nil {
		r.warnFn(err)
	}
	selectedNotes := SelectNotesForContext(notes, rankedIDs, r.tokenBudget)
	memoryBlock := BuildMemoryBlock(selectedNotes)

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
		sourceIDs, _ := json.Marshal(noteIDs(selectedNotes))
		promptHash := sha256.Sum256([]byte(systemPrompt))
		_ = r.cacheRepo.Upsert(ctx, &store.ContextCacheEntry{
			ID:            fmt.Sprintf("cc-%x", time.Now().UnixNano()),
			ProfileID:     profileID,
			CacheKey:      cacheKey,
			Payload:       string(payload),
			SourceNoteIDs: string(sourceIDs),
			PromptVersion: fmt.Sprintf("prompt:%x", promptHash),
			StateVersion:  summaryID,
			CreatedAt:     time.Now().UTC(),
			ExpiresAt:     &expires,
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

// SelectNotesForContext orders notes by relevance (rankedIDs) and enforces token budget.
// If rankedIDs is empty, notes are assumed pre-sorted by importance then recency.
func SelectNotesForContext(notes []*store.MemoryNote, rankedIDs []string, budget int) []*store.MemoryNote {
	if len(notes) == 0 {
		return nil
	}
	if budget <= 0 {
		budget = 1500
	}

	ordered := notes
	if len(rankedIDs) > 0 {
		byID := make(map[string]*store.MemoryNote, len(notes))
		for _, n := range notes {
			byID[n.ID] = n
		}
		ordered = make([]*store.MemoryNote, 0, len(rankedIDs))
		for _, id := range rankedIDs {
			if note, ok := byID[id]; ok {
				ordered = append(ordered, note)
			}
		}
	} else {
		ordered = make([]*store.MemoryNote, len(notes))
		copy(ordered, notes)
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].Importance == ordered[j].Importance {
				return ordered[i].CreatedAt.After(ordered[j].CreatedAt)
			}
			return ordered[i].Importance > ordered[j].Importance
		})
	}

	selected := make([]*store.MemoryNote, 0, len(ordered))
	used := 0
	for _, note := range ordered {
		cost := estimateTokens(note.Content)
		if used+cost > budget {
			break
		}
		selected = append(selected, note)
		used += cost
	}
	return selected
}

func estimateTokens(content string) int {
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return 1
	}
	return len(fields)
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

func cacheKeyFor(profileID, systemPrompt, summaryID string, tokenBudget int) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s::%s::%s::%d", profileID, systemPrompt, summaryID, tokenBudget)))
	return fmt.Sprintf("ctx:%x", hash)
}

func noteIDs(notes []*store.MemoryNote) []string {
	ids := make([]string, 0, len(notes))
	for _, note := range notes {
		ids = append(ids, note.ID)
	}
	return ids
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

type noteLister struct {
	repo *store.MemoryNoteRepo
}

func (n noteLister) ListByProfile(ctx context.Context, profileID string) ([]vector.MemoryNoteRecord, error) {
	notes, err := n.repo.ListByProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	out := make([]vector.MemoryNoteRecord, 0, len(notes))
	for _, note := range notes {
		out = append(out, vector.MemoryNoteRecord{ID: note.ID, Content: note.Content})
	}
	return out, nil
}

func (r *Retrieval) rankNotes(ctx context.Context, systemPrompt, summaryText string) ([]string, error) {
	if r.vectorIndex == nil || r.embedder == nil || r.profileID == "" {
		return nil, nil
	}
	if err := r.checkVectorIndex(); err != nil {
		return nil, err
	}
	queryText := strings.TrimSpace(systemPrompt)
	if summaryText != "" {
		queryText = queryText + "\n" + summaryText
	}
	if queryText == "" {
		return nil, nil
	}
	model := r.embeddingModel
	resp, err := r.embedder.Embed(ctx, provider.EmbeddingRequest{Input: queryText, Model: model})
	if err != nil {
		return nil, err
	}

	manifestRepo := store.NewVectorManifestRepo(r.noteRepo.DB())
	entries, err := manifestRepo.ListEntries(ctx, r.profileID)
	if err != nil {
		return nil, err
	}
	vectorEntries := make([]vector.Entry, 0, len(entries))
	for _, e := range entries {
		vectorEntries = append(vectorEntries, vector.Entry{
			ID:             e.ID,
			ProfileID:      e.ProfileID,
			SourceType:     vector.SourceType(e.SourceType),
			SourceID:       e.SourceID,
			ChunkHash:      e.ChunkHash,
			EmbeddingModel: e.EmbeddingModel,
			EmbeddingDim:   e.EmbeddingDim,
			VectorRef:      e.VectorRef,
		})
	}
	if fileIndex, ok := r.vectorIndex.(*vector.FileIndex); ok {
		fileIndex.WithProfile(r.profileID)
		fileIndex.SeedEntries(vectorEntries)
		if err := fileIndex.Load(); err != nil {
			return nil, err
		}
	}

	retrieval := vector.NewHybridRetrieval(r.vectorIndex, noteLister{repo: r.noteRepo}, r.profileID)
	results, err := retrieval.Retrieve(ctx, resp.Embedding, vectorTopK)
	if err != nil {
		return nil, err
	}
	ranked := make([]string, 0, len(results))
	for _, res := range results {
		ranked = append(ranked, res.Note.ID)
	}
	return ranked, nil
}

package vector

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"noto/internal/provider"
	"noto/internal/startup"
)

// ManifestStatusSetter is the interface for updating the vector manifest status.
// It accepts a string to avoid type coupling with the store package.
type ManifestStatusSetter interface {
	SetManifestStatusStr(ctx context.Context, profileID string, status string) error
}

// Rebuilder rebuilds the vector index from a set of notes.
type Rebuilder struct {
	manifestSetter ManifestStatusSetter
	manifestRepo   ManifestEntryRepo
	index          Index
	profileID      string
	embedder       Embedder
	embeddingModel string
	progress       startup.Reporter
	validate       func(context.Context) error
	previousStatus string
}

type manifestRebuildRepository interface {
	GetEmbeddingState(context.Context, string) (string, int, string, error)
	ReplaceEntriesAndEmbedding(context.Context, string, string, int, []*ManifestEntry) error
}

type indexPreparer interface {
	PrepareRebuild([]Entry, string) (*PreparedRebuild, error)
}

// WithProgress configures startup progress reporting.
func (rb *Rebuilder) WithProgress(report startup.Reporter) *Rebuilder {
	rb.progress = report
	return rb
}

// NewRebuilder creates a Rebuilder.
func NewRebuilder(
	manifestSetter ManifestStatusSetter,
	index Index,
	profileID string,
) *Rebuilder {
	return &Rebuilder{
		manifestSetter: manifestSetter,
		index:          index,
		profileID:      profileID,
	}
}

// WithManifest sets the manifest repo for rebuilt entries.
func (rb *Rebuilder) WithManifest(repo ManifestEntryRepo) *Rebuilder {
	rb.manifestRepo = repo
	return rb
}

// WithEmbedder sets the embedder and model for rebuild.
func (rb *Rebuilder) WithEmbedder(embedder Embedder, model string) *Rebuilder {
	rb.embedder = embedder
	rb.embeddingModel = model
	return rb
}

// WithValidator checks that source records have not changed during a long rebuild.
func (rb *Rebuilder) WithValidator(validate func(context.Context) error) *Rebuilder {
	rb.validate = validate
	return rb
}

// WithPreviousStatus sets the manifest status to restore on pre-publication failure.
func (rb *Rebuilder) WithPreviousStatus(status string) *Rebuilder {
	rb.previousStatus = status
	return rb
}

// Rebuild discards the existing index and re-indexes the given notes.
func (rb *Rebuilder) Rebuild(ctx context.Context, notes []MemoryNoteRecord) (resultErr error) {
	event := startup.Event{Phase: "rebuild", State: "start", Total: len(notes)}
	rb.progress.Send(event)
	defer func() {
		event.State = "success"
		if resultErr != nil {
			event.State = "failure"
		}
		rb.progress.Send(event)
	}()
	if len(notes) > 0 && rb.embedder == nil {
		return errors.New("vector: embedder not configured")
	}

	entries := make([]Entry, 0, len(notes))
	for _, note := range notes {
		resp, err := rb.embedder.Embed(ctx, provider.EmbeddingRequest{Input: note.Content, Model: rb.embeddingModel})
		if err != nil {
			return fmt.Errorf("vector: embed note %s: %w", note.ID, err)
		}
		entry := Entry{
			ID:             "ve-" + note.ID,
			ProfileID:      rb.profileID,
			SourceType:     SourceMemoryNote,
			SourceID:       note.ID,
			ChunkHash:      ContentHash(note.Content),
			EmbeddingModel: rb.embeddingModel,
			EmbeddingDim:   len(resp.Embedding),
			Vector:         resp.Embedding,
		}
		entries = append(entries, entry)
		event.Done++
		event.State = "update"
		rb.progress.Send(event)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if rb.validate != nil {
		if err := rb.validate(ctx); err != nil {
			return fmt.Errorf("vector: sources changed during rebuild: %w", err)
		}
	}

	var prepared *PreparedRebuild
	if preparer, ok := rb.index.(indexPreparer); ok {
		var err error
		prepared, err = preparer.PrepareRebuild(entries, rb.embeddingModel)
		if err != nil {
			return fmt.Errorf("vector: prepare index rebuild: %w", err)
		}
		defer prepared.Abort()
	}
	if rb.validate != nil {
		if err := rb.validate(ctx); err != nil {
			return fmt.Errorf("vector: sources changed during rebuild: %w", err)
		}
	}

	previousStatus := rb.previousStatus
	if previousStatus == "" {
		previousStatus = string(ManifestReady)
	}
	manifestRepo, transactionalManifest := rb.manifestRepo.(manifestRebuildRepository)
	var previousModel string
	var previousDim int
	var previousEntries []*ManifestEntry
	if transactionalManifest && prepared != nil {
		var err error
		previousModel, previousDim, previousStatus, err = manifestRepo.GetEmbeddingState(ctx, rb.profileID)
		if err != nil {
			return fmt.Errorf("vector: read manifest state: %w", err)
		}
		previousEntries, err = rb.manifestRepo.ListEntriesVec(ctx, rb.profileID)
		if err != nil {
			return fmt.Errorf("vector: read manifest entries: %w", err)
		}
	}
	if previousStatus == "" {
		previousStatus = string(ManifestReady)
	}
	if err := rb.manifestSetter.SetManifestStatusStr(ctx, rb.profileID, string(ManifestRebuilding)); err != nil {
		return fmt.Errorf("vector: mark manifest rebuilding: %w", err)
	}

	manifestEntries := make([]*ManifestEntry, 0, len(entries))
	for i, entry := range entries {
		manifestEntries = append(manifestEntries, &ManifestEntry{
			ID: entry.ID, ProfileID: entry.ProfileID, SourceType: entry.SourceType,
			SourceID: entry.SourceID, ChunkHash: entry.ChunkHash,
			EmbeddingModel: entry.EmbeddingModel, EmbeddingDim: entry.EmbeddingDim,
			VectorRef: strconv.Itoa(i),
		})
	}
	if transactionalManifest && prepared != nil {
		dim := 0
		if len(entries) > 0 {
			dim = entries[0].EmbeddingDim
		}
		if err := manifestRepo.ReplaceEntriesAndEmbedding(ctx, rb.profileID, rb.embeddingModel, dim, manifestEntries); err != nil {
			_ = rb.setStatusDetached(ctx, previousStatus)
			return fmt.Errorf("vector: prepare manifest replacement: %w", err)
		}
		if err := prepared.Commit(); err != nil {
			detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			restoreErr := manifestRepo.ReplaceEntriesAndEmbedding(detached, rb.profileID, previousModel, previousDim, previousEntries)
			if restoreErr == nil {
				restoreErr = rb.manifestSetter.SetManifestStatusStr(detached, rb.profileID, previousStatus)
			}
			if restoreErr != nil {
				_ = rb.manifestSetter.SetManifestStatusStr(detached, rb.profileID, string(ManifestFailed))
			}
			cancel()
			return fmt.Errorf("vector: publish index rebuild: %w", err)
		}
		// A failed status update is recoverable at startup; the published index
		// and manifest already agree on the embedding space.
		_ = rb.setStatusDetached(ctx, string(ManifestReady))
		return nil
	}

	if prepared != nil {
		if err := prepared.Commit(); err != nil {
			_ = rb.setStatusDetached(ctx, string(ManifestFailed))
			return fmt.Errorf("vector: publish index rebuild: %w", err)
		}
	} else if err := rb.index.Rebuild(entries); err != nil {
		_ = rb.setStatusDetached(ctx, string(ManifestFailed))
		return fmt.Errorf("vector: rebuild index: %w", err)
	} else if err := rb.index.Flush(); err != nil {
		_ = rb.setStatusDetached(ctx, string(ManifestFailed))
		return fmt.Errorf("vector: flush after rebuild: %w", err)
	}
	if rb.manifestRepo != nil {
		for _, entry := range manifestEntries {
			if err := rb.manifestRepo.UpsertEntry(ctx, entry); err != nil {
				_ = rb.setStatusDetached(ctx, string(ManifestFailed))
				return fmt.Errorf("vector: update manifest for %s: %w", entry.SourceID, err)
			}
		}
	}
	if setter, ok := rb.manifestSetter.(interface {
		SetManifestEmbeddingModel(context.Context, string, string, int) error
	}); ok {
		dim := 0
		if len(entries) > 0 {
			dim = entries[0].EmbeddingDim
		}
		if err := setter.SetManifestEmbeddingModel(ctx, rb.profileID, rb.embeddingModel, dim); err != nil {
			_ = rb.setStatusDetached(ctx, string(ManifestFailed))
			return fmt.Errorf("vector: update manifest embedding model: %w", err)
		}
	}
	_ = rb.setStatusDetached(ctx, string(ManifestReady))
	return nil
}

func (rb *Rebuilder) setStatusDetached(ctx context.Context, status string) error {
	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return rb.manifestSetter.SetManifestStatusStr(detached, rb.profileID, status)
}

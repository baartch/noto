package vector

import (
	"context"
	"fmt"
)

// ManifestSetter is a minimal interface to avoid circular imports with the store package.
// It accepts a string status to avoid type coupling.
type ManifestSetter interface {
	SetManifestStatusStr(ctx context.Context, profileID string, status string) error
}

// InvalidationTriggers handles vector index invalidation when profile data changes.
type InvalidationTriggers struct {
	manifestRepo ManifestSetter
	profileID    string
}

// NewInvalidationTriggers creates an InvalidationTriggers for the given profile.
func NewInvalidationTriggers(manifestRepo ManifestSetter, profileID string) *InvalidationTriggers {
	return &InvalidationTriggers{manifestRepo: manifestRepo, profileID: profileID}
}

// OnPromptChange marks the vector index as stale when the system prompt changes.
// This triggers a rebuild on next use since the prompt affects semantic context.
func (t *InvalidationTriggers) OnPromptChange(ctx context.Context) error {
	if err := t.manifestRepo.SetManifestStatusStr(ctx, t.profileID, string(ManifestStale)); err != nil {
		return fmt.Errorf("vector: mark stale on prompt change: %w", err)
	}
	return nil
}

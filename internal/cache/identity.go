package cache

import (
	"crypto/sha256"
	"fmt"
)

// Identity defines cache-hit safe dimensions.
type Identity struct {
	ProfileID      string
	Prompt         string
	NotesHash      string
	TokenBudget    int
	EmbeddingModel string
}

// Key returns the deterministic cache key for the identity.
func (i Identity) Key() string {
	buf := fmt.Appendf(nil, "%s::%s::%s::%d::%s", i.ProfileID, i.Prompt, i.NotesHash, i.TokenBudget, i.EmbeddingModel)
	h := sha256.Sum256(buf)
	return fmt.Sprintf("ctx:%x", h)
}

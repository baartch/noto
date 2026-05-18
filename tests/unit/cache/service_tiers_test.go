package cache_test

import (
	"testing"

	"noto/internal/memory"
	"noto/internal/store"
)

func TestPlaceholder_TierCoverage(t *testing.T) {
	_ = memory.RetrievalContext{CacheTier: "l1"}
	_ = store.ContextCacheEntry{}
}

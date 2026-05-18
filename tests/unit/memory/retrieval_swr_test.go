package memory_test

import (
	"testing"

	"noto/internal/memory"
)

func TestRetrievalContext_SWRMetadataFieldsAvailable(t *testing.T) {
	rc := memory.RetrievalContext{ServedStale: true, RevalidationStarted: true, CacheTier: "l2"}
	if !rc.ServedStale || !rc.RevalidationStarted || rc.CacheTier == "" {
		t.Fatalf("expected SWR metadata fields to be available")
	}
}

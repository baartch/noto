package memory_test

import (
	"testing"

	"noto/internal/memory"
)

func TestRetrievalContext_MissReasonFieldAvailable(t *testing.T) {
	rc := memory.RetrievalContext{MissReason: "not_found"}
	if rc.MissReason == "" {
		t.Fatalf("expected miss reason")
	}
}

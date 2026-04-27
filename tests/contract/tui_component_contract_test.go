package contract

import (
	"os"
	"strings"
	"testing"
)

func TestTUIFlowContract_ContainsComponentAndRationaleRequirements(t *testing.T) {
	b, err := os.ReadFile("../../specs/004-bubbletea-tui/contracts/tui-flows.md")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	s := string(b)
	required := []string{
		"If a suitable Bubbles component exists, it must be used.",
		"If a custom component is used, its rationale is documented",
		"Token/cost totals include contributions from main chat model, extractor model, and embeddings model operations.",
	}
	for _, r := range required {
		if !strings.Contains(s, r) {
			t.Fatalf("contract missing requirement: %q", r)
		}
	}
}

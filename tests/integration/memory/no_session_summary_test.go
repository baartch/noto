package integration

import (
	"strings"
	"testing"

	"noto/internal/memory"
)

func TestAssembledPrompt_ExcludesConversationSummaries(t *testing.T) {
	assembled := memory.AssemblePrompt("system", "", "## Raw Notes\n- [fact] current note")
	if strings.Contains(assembled, "legacy session summary") {
		t.Fatalf("did not expect legacy session summary in assembled prompt: %q", assembled)
	}
	if strings.Contains(assembled, "Previous Session Summary") {
		t.Fatalf("did not expect previous session section in assembled prompt: %q", assembled)
	}
}

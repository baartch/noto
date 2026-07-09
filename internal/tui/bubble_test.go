package tui

import (
	"strings"
	"testing"
	"time"
)

func TestRenderUserBubble_PreservesLineBreaksAndIndentation(t *testing.T) {
	content := "first line\n  indented second line\n    indented third line"

	rendered := renderUserBubble(content, "You", time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC), 80)

	if !strings.Contains(rendered, "first line") {
		t.Fatalf("expected first line in rendered bubble")
	}
	if !strings.Contains(rendered, "  indented second line") {
		t.Fatalf("expected second line indentation to be preserved")
	}
	if !strings.Contains(rendered, "    indented third line") {
		t.Fatalf("expected third line indentation to be preserved")
	}

	if strings.Contains(rendered, "first line indented second line") {
		t.Fatalf("expected explicit newline between first and second lines to be preserved")
	}
	if strings.Contains(rendered, "indented second line indented third line") {
		t.Fatalf("expected explicit newline between second and third lines to be preserved")
	}
}

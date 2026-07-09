package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"

	"noto/internal/store"
)

func TestSidebar_JoinWidths(t *testing.T) {
	width := 200
	height := 40
	sidebarW := max(width/5, 36)
	mainWidth := width - sidebarW - 1

	vp := viewport.New(
		viewport.WithWidth(mainWidth),
		viewport.WithHeight(height-4),
	)
	vp.SetContent(strings.Repeat("Hello world.\n", 30))

	sidebar := newSidebar(sidebarW, nil, nil, "")
	sidebar.open = true
	sidebar.width = sidebarW

	content := vp.View() + "\n" +
		"─────────────────────────────────────────────────────\n" +
		"  some input line here\n" +
		"  profile  [model]  v1.0  ctrl+h help"

	sideContent := sidebar.render(height)

	mainLines := strings.Split(content, "\n")
	sideLines := strings.Split(sideContent, "\n")
	n := max(len(mainLines), len(sideLines))

	var sb strings.Builder
	for i := range n {
		var ml, sl string
		if i < len(mainLines) {
			ml = mainLines[i]
		}
		if i < len(sideLines) {
			sl = sideLines[i]
		}
		w := lipgloss.Width(ml)
		if w < mainWidth {
			ml += strings.Repeat(" ", mainWidth-w)
		}
		sb.WriteString(ml)
		sb.WriteString(sl)
		if i < n-1 {
			sb.WriteString("\n")
		}
	}

	joined := sb.String()
	joinedLines := strings.Split(joined, "\n")
	overCount := 0
	maxExcess := 0
	for _, line := range joinedLines {
		lw := lipgloss.Width(line)
		if lw > width {
			overCount++
			if lw-width > maxExcess {
				maxExcess = lw - width
			}
		}
	}
	if overCount > 0 {
		t.Errorf("%d lines exceed terminal width %d (max excess %d cols)", overCount, width, maxExcess)
	}
}

func TestSidebar_AllLinesEqualWidth(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	s.width = 36
	s.activeTab = sidebarTabNotes
	s.notes = []*store.MemoryNote{
		{ID: "n1", Category: store.CategoryFact, Content: "short", Importance: 5, CreatedAt: time.Now()},
	}
	rendered := s.render(40)
	lines := strings.Split(rendered, "\n")

	for i, line := range lines {
		w := lipgloss.Width(line)
		if w != s.width {
			t.Errorf("line %d: width=%d, want %d", i, w, s.width)
		}
	}
}

func TestSidebar_RenderSmoke(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	s.width = 36
	rendered := s.render(40)
	if rendered == "" {
		t.Fatal("sidebar.render() returned empty string")
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatal("too few lines")
	}
	// First line should have visible tab text
	if lipgloss.Width(lines[0]) < 10 {
		t.Errorf("first line too narrow: %d", lipgloss.Width(lines[0]))
	}
	// Verify border is visible
	hasBorder := false
	for _, l := range lines {
		if lipgloss.Width(l) > 0 {
			hasBorder = true
			break
		}
	}
	if !hasBorder {
		t.Error("all lines empty")
	}
	_ = time.Now
}

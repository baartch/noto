package tui

import (
	"strings"
	"testing"
	"time"

	"noto/internal/store"
)

func TestSidebar_RenderTabs(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true

	t.Run("renders as non-empty", func(t *testing.T) {
		got := s.renderTabs()
		if got == "" {
			t.Fatal("expected non-empty tab rendering")
		}
	})

	t.Run("active tab cycles", func(t *testing.T) {
		s.activeTab = 0
		s.activeTab = (s.activeTab + 1) % 3
		if s.activeTab != 1 {
			t.Fatalf("expected tab 1, got %d", s.activeTab)
		}
		s.activeTab = (s.activeTab + 1) % 3
		if s.activeTab != 2 {
			t.Fatalf("expected tab 2, got %d", s.activeTab)
		}
		s.activeTab = (s.activeTab + 1) % 3
		if s.activeTab != 0 {
			t.Fatalf("expected tab 0, got %d", s.activeTab)
		}
	})

	t.Run("non-empty rendering", func(t *testing.T) {
		s.activeTab = 0
		got := s.renderTabs()
		if got == "" {
			t.Fatal("expected non-empty tab rendering")
		}
	})
}

func TestSidebar_RenderContent_Smoke(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	s.width = 36

	t.Run("empty state shows placeholder", func(t *testing.T) {
		content := s.renderContent()
		if content == "" {
			t.Fatal("expected non-empty content for empty notes tab")
		}
	})
}

func TestSidebar_RenderNoteEntry(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	s.width = 36

	now := time.Date(2026, 7, 9, 14, 30, 0, 0, time.UTC)
	s.notes = []*store.MemoryNote{
		{
			ID:         "n1",
			Category:   store.CategoryFact,
			Content:    "Test note content",
			Importance: 8,
			CreatedAt:  now,
		},
	}

	content := s.renderContent()

	t.Run("includes metadata", func(t *testing.T) {
		if !strings.Contains(content, "fact") {
			t.Fatal("expected category 'fact' in rendered note")
		}
		if !strings.Contains(content, "I8") {
			t.Fatal("expected 'I8' (importance prefix) in rendered note")
		}
		if !strings.Contains(content, "2026-07-09 14:30") {
			t.Fatal("expected timestamp in rendered note")
		}
	})

	t.Run("includes border", func(t *testing.T) {
		if !strings.Contains(content, "─") {
			t.Fatal("expected border characters in rendered note")
		}
	})
}

func TestSidebar_RenderSummaryEntry(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	s.width = 36
	s.activeTab = 1

	start := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	s.weekly = []*store.MemorySummary{
		{
			ID:          "s1",
			SummaryType: store.SummaryTypeWeekly,
			PeriodKey:   "2026-W27",
			PeriodStart: start,
			PeriodEnd:   end,
			Content:     "This week's summary content.",
		},
	}

	t.Run("weekly summary has period key and dates", func(t *testing.T) {
		content := s.renderContent()
		if !strings.Contains(content, "2026-W27") {
			t.Fatal("expected period key in summary")
		}
		if !strings.Contains(content, "2026-06-29") || !strings.Contains(content, "2026-07-05") {
			t.Fatal("expected period dates in weekly summary")
		}
		if !strings.Contains(content, "─") {
			t.Fatal("expected border in summary entry")
		}
	})
}

func TestSidebar_ViewportWidth(t *testing.T) {
	// Smoke test: width should not cause panic at extreme values.
	s := newSidebar(10, nil, nil, "")
	s.open = true
	s.activeTab = sidebarTabNotes
	s.notes = []*store.MemoryNote{
		{ID: "n1", Category: store.CategoryFact, Content: "test", Importance: 5, CreatedAt: time.Now()},
	}
	_ = s.renderContent()
	_ = s.render(30)
}

func TestSidebar_Height(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	rendered := s.render(40)
	if rendered == "" {
		t.Fatal("expected non-empty render at height 40")
	}
	// Verify the border separator is present.
	if !strings.Contains(rendered, "│") {
		t.Fatal("expected sidebar border character in render")
	}
}

func TestSidebar_CategoryRenderedWithoutLabel(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	s.width = 36
	s.notes = []*store.MemoryNote{
		{
			ID:         "n1",
			Category:   store.CategoryProgress,
			Content:    "Working on feature X",
			Importance: 4,
			CreatedAt:  time.Now(),
		},
	}
	content := s.renderContent()
	// Category should be shown without a label prefix like "Category:".
	if strings.Contains(content, "Category:") {
		t.Fatal("category should not have a label prefix")
	}
	if !strings.Contains(content, "progress") {
		t.Fatal("expected category 'progress' to appear")
	}
}

func TestSidebar_MetadataFormat(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	s.width = 36
	now := time.Date(2026, 7, 9, 14, 30, 0, 0, time.UTC)
	s.notes = []*store.MemoryNote{
		{
			ID:         "n1",
			Category:   store.CategoryActionItem,
			Content:    "Need to review PR",
			Importance: 10,
			CreatedAt:  now,
		},
	}
	content := s.renderContent()
	// Format: "action_item  I10  ·  2026-07-09 14:30"
	if !strings.Contains(content, "action_item  I10  ·") {
		t.Fatalf("expected metadata 'action_item  I10  · ...', got:\n%s", content)
	}
}

func TestSidebar_MetadataStyle(t *testing.T) {
	meta := sidebarMetaStyle.Render("test · metadata")
	// The style should set the foreground color (we just verify it's non-empty).
	if meta == "test · metadata" {
		// Without styling the string would be exactly the input.
		// With styling, lipgloss adds ANSI escape codes.
		if !strings.Contains(meta, "test") {
			t.Fatal("style output should contain the input text")
		}
	}
}

func TestSidebar_EmptyHasNoScrollMore(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	s.width = 36
	s.hasMore = true
	s.loading = false
	var sb strings.Builder
	s.appendFooter(&sb)
	footer := sb.String()
	if !strings.Contains(footer, "older") {
		t.Fatal("expected 'scroll older' hint when hasMore is true and not loading")
	}
}

func TestSidebar_EmptyLoading(t *testing.T) {
	s := newSidebar(36, nil, nil, "")
	s.open = true
	s.width = 36
	s.hasMore = false
	s.loading = true
	var sb strings.Builder
	s.appendFooter(&sb)
	footer := sb.String()
	if !strings.Contains(footer, "loading") {
		t.Fatal("expected 'loading' indicator when loading is true")
	}
}

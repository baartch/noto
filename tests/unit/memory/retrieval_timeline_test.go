package memory_test

import (
	"strings"
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/store"
)

func TestBuildTimelineMemoryBlock_FormatsSeparateSections(t *testing.T) {
	raw := []*store.MemoryNote{{ID: "n1", Category: store.CategoryFact, Content: "recent note"}}
	weekly := []*store.MemorySummary{{ID: "w1", SummaryType: store.SummaryTypeWeekly, PeriodKey: "2026-W18", Content: "weekly summary"}}
	monthly := []*store.MemorySummary{{ID: "m1", SummaryType: store.SummaryTypeMonthly, PeriodKey: "2026-03", Content: "monthly summary"}}

	block := memory.BuildTimelineMemoryBlock(raw, weekly, monthly)
	if !strings.Contains(block, "## Raw Notes") {
		t.Fatalf("expected raw section, got %q", block)
	}
	if !strings.Contains(block, "## Weekly Summaries") {
		t.Fatalf("expected weekly section, got %q", block)
	}
	if !strings.Contains(block, "## Monthly Summaries") {
		t.Fatalf("expected monthly section, got %q", block)
	}
	if !strings.Contains(block, "recent note") || !strings.Contains(block, "weekly summary") || !strings.Contains(block, "monthly summary") {
		t.Fatalf("expected all content in block, got %q", block)
	}
}

func TestBuildTimelineMemoryBlock_OmitsEmptySections(t *testing.T) {
	block := memory.BuildTimelineMemoryBlock(nil, nil, nil)
	if block != "" {
		t.Fatalf("expected empty block, got %q", block)
	}
}

func TestComputeTimelineWindow_WeeklyLayerExtendsToMonthlyBoundary(t *testing.T) {
	now := time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC)
	settings := &store.TimelineSettings{ProfileID: "p1", RawNoteDays: 5, WeeklySummaryWeeks: 1, MonthlySummaryMonths: store.MonthlySummaryAllRemaining}
	window, err := memory.ComputeTimelineWindow(now, settings)
	if err != nil {
		t.Fatalf("compute window: %v", err)
	}
	if got, want := window.RawStart, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("raw start = %v, want %v", got, want)
	}
	if got, want := window.WeeklyStart, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("weekly start = %v, want %v", got, want)
	}
	if got, want := window.MonthlyStart, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("monthly start = %v, want %v", got, want)
	}
}

func TestAssemblePrompt_NoSessionSummarySection(t *testing.T) {
	assembled := memory.AssemblePrompt("system", "", "## Raw Notes\n- [fact] hello")
	if strings.Contains(assembled, "Previous Session Summary") {
		t.Fatalf("did not expect session summary section in %q", assembled)
	}
	if !strings.Contains(assembled, "## Raw Notes") {
		t.Fatalf("expected memory block in assembled prompt: %q", assembled)
	}
}

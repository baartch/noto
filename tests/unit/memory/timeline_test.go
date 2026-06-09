package memory_test

import (
	"testing"
	"time"

	"noto/internal/memory"
	"noto/internal/store"
)

func TestComputeTimelineWindow_FillsBackToMonday(t *testing.T) {
	now := time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC) // Tuesday
	settings := &store.TimelineSettings{ProfileID: "p1", RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: store.MonthlySummaryAllRemaining}
	window, err := memory.ComputeTimelineWindow(now, settings)
	if err != nil {
		t.Fatalf("compute window: %v", err)
	}
	if got, want := window.RawStart.Weekday(), time.Monday; got != want {
		t.Fatalf("raw start weekday = %v, want %v", got, want)
	}
	if !window.WeeklyEnd.Equal(window.RawStart) {
		t.Fatalf("weekly end = %v, want raw start %v", window.WeeklyEnd, window.RawStart)
	}
	if window.MonthlyCutoff != nil {
		t.Fatalf("expected nil monthly cutoff for all remaining")
	}
}

func TestComputeTimelineWindow_BoundedMonthlyCutoff(t *testing.T) {
	now := time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC)
	settings := &store.TimelineSettings{ProfileID: "p1", RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: 3}
	window, err := memory.ComputeTimelineWindow(now, settings)
	if err != nil {
		t.Fatalf("compute window: %v", err)
	}
	if window.MonthlyCutoff == nil {
		t.Fatalf("expected bounded monthly cutoff")
	}
	if got := window.MonthlyCutoff.Month(); got != time.Month(12) {
		t.Fatalf("cutoff month = %v, want December", got)
	}
}

func TestComputeTimelineWindow_RejectsZeroValueLayers(t *testing.T) {
	cases := []*store.TimelineSettings{
		{ProfileID: "p1", RawNoteDays: 0, WeeklySummaryWeeks: 8, MonthlySummaryMonths: store.MonthlySummaryAllRemaining},
		{ProfileID: "p1", RawNoteDays: 30, WeeklySummaryWeeks: 0, MonthlySummaryMonths: store.MonthlySummaryAllRemaining},
		{ProfileID: "p1", RawNoteDays: 30, WeeklySummaryWeeks: 8, MonthlySummaryMonths: 0},
	}
	for _, tc := range cases {
		if _, err := memory.ComputeTimelineWindow(time.Now().UTC(), tc); err == nil {
			t.Fatalf("expected validation error for %#v", tc)
		}
	}
}

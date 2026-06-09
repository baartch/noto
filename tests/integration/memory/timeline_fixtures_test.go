package integration

import (
	"time"

	"noto/internal/store"
)

// TimelineFixtureNote is a lightweight fixture row used by timeline assembly tests.
type TimelineFixtureNote struct {
	ID         string
	ProfileID  string
	Category   store.MemoryCategory
	Content    string
	Importance int
	CreatedAt  time.Time
}

// TimelineFixtureSummary is a lightweight fixture row used by rollup and time-range tests.
type TimelineFixtureSummary struct {
	ID         string
	ProfileID  string
	Type       string
	PeriodKey  string
	Content    string
	PeriodStart time.Time
	PeriodEnd   time.Time
}

// TimelineFixtureSettings captures default and override values used by timeline tests.
type TimelineFixtureSettings struct {
	RawNoteDays          int
	WeeklySummaryWeeks   int
	MonthlySummaryMonths int
}

func DefaultTimelineFixtureSettings() TimelineFixtureSettings {
	return TimelineFixtureSettings{
		RawNoteDays:          30,
		WeeklySummaryWeeks:   8,
		MonthlySummaryMonths: store.MonthlySummaryAllRemaining,
	}
}

func FixtureTimelineNow() time.Time {
	return time.Date(2026, 6, 9, 15, 0, 0, 0, time.UTC)
}

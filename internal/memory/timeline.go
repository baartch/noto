package memory

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"noto/internal/store"
)

// TimelineWindow describes the computed raw, weekly, and monthly time ranges
// used for assembled memory context.
type TimelineWindow struct {
	RawStart      time.Time
	RawEnd        time.Time
	WeeklyStart   time.Time
	WeeklyEnd     time.Time
	MonthlyStart  time.Time
	MonthlyCutoff *time.Time
}

// ComputeTimelineWindow derives the effective timeline ranges from the current
// time and persisted profile timeline settings.
func ComputeTimelineWindow(now time.Time, settings *store.TimelineSettings) (TimelineWindow, error) {
	if settings == nil {
		return TimelineWindow{}, errors.New("memory: timeline settings are nil")
	}
	if err := settings.Validate(); err != nil {
		return TimelineWindow{}, err
	}

	end := now.UTC()
	rawStart := end.AddDate(0, 0, -settings.RawNoteDays)
	rawStart = precedingMonday(rawStart)
	weeklyEnd := rawStart
	weeklyStart := startOfWeek(weeklyEnd).AddDate(0, 0, -7*settings.WeeklySummaryWeeks)
	monthlyStart := firstDayOfMonth(weeklyStart)
	if weeklyStart.After(monthlyStart) {
		weeklyStart = monthlyStart
	}

	var cutoff *time.Time
	if settings.MonthlySummaryMonths != store.MonthlySummaryAllRemaining {
		c := firstDayOfMonth(monthlyStart).AddDate(0, -settings.MonthlySummaryMonths, 0)
		cutoff = &c
	}

	return TimelineWindow{
		RawStart:      rawStart,
		RawEnd:        end,
		WeeklyStart:   weeklyStart,
		WeeklyEnd:     weeklyEnd,
		MonthlyStart:  monthlyStart,
		MonthlyCutoff: cutoff,
	}, nil
}

func precedingMonday(t time.Time) time.Time {
	t = startOfDay(t.UTC())
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}

func startOfWeek(t time.Time) time.Time {
	return precedingMonday(t)
}

func firstDayOfMonth(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func startOfDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// MonthlySummaryMonthsString serializes a monthly summary window value.
func MonthlySummaryMonthsString(v int) string {
	if v == store.MonthlySummaryAllRemaining {
		return "all_remaining"
	}
	return strconv.Itoa(v)
}

// ParseMonthlySummaryMonths parses a serialized monthly summary window value.
func ParseMonthlySummaryMonths(v string) (int, error) {
	if v == "all_remaining" {
		return store.MonthlySummaryAllRemaining, nil
	}
	return 0, fmt.Errorf("memory: unsupported monthly summary value %q", v)
}

package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"noto/internal/cache"
	"noto/internal/store"
)

// SummaryStateVersion derives a stable version fingerprint for the notes that
// feed a summary artifact.
func SummaryStateVersion(noteIDs []string) string {
	h := sha256.New()
	for _, id := range noteIDs {
		_, _ = h.Write([]byte(id))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// WeeklyPeriodKey returns the canonical ISO week key for a timestamp.
func WeeklyPeriodKey(t time.Time) string {
	start := startOfWeek(t)
	year, week := start.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", year, week)
}

// MonthlyPeriodKey returns the canonical calendar month key for a timestamp.
func MonthlyPeriodKey(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%04d-%02d", t.Year(), int(t.Month()))
}

// IsSummaryFresh reports whether a stored summary matches the expected source
// state and freshness marker.
func IsSummaryFresh(summary *store.MemorySummary, stateVersion string) bool {
	if summary == nil {
		return false
	}
	return summary.FreshnessState == store.SummaryFresh && summary.SourceStateVersion == stateVersion
}

// RollupCreationCounts reports how many summaries were created during a catch-up pass.
type RollupCreationCounts struct {
	Weekly  int
	Monthly int
}

// SummaryRollupBuilder creates, marks, and regenerates weekly/monthly summary artifacts.
type SummaryRollupBuilder struct {
	noteRepo          *store.MemoryNoteRepo
	summaryRepo       *store.MemorySummaryRepo
	cacheInvalidator  interface{ OnSummaryChange(context.Context, string) error }
}

// NewSummaryRollupBuilder creates a rollup builder.
func NewSummaryRollupBuilder(noteRepo *store.MemoryNoteRepo, summaryRepo *store.MemorySummaryRepo) *SummaryRollupBuilder {
	builder := &SummaryRollupBuilder{noteRepo: noteRepo, summaryRepo: summaryRepo}
	if noteRepo != nil && noteRepo.DB() != nil {
		builder.cacheInvalidator = cache.NewInvalidationTriggers(store.NewContextCacheRepo(noteRepo.DB()))
	}
	return builder
}

// CatchUp creates any missing completed weekly/monthly summaries for a profile.
func (b *SummaryRollupBuilder) CatchUp(ctx context.Context, profileID string, now time.Time) (RollupCreationCounts, error) {
	counts := RollupCreationCounts{}
	notes, err := b.noteRepo.ListByProfile(ctx, profileID)
	if err != nil {
		return counts, fmt.Errorf("memory: list notes for rollups: %w", err)
	}

	weeklyGroups := groupNotesByCompletedWeek(notes, now)
	for key, grouped := range weeklyGroups {
		if _, err := b.summaryRepo.GetByPeriod(ctx, profileID, store.SummaryTypeWeekly, key); err == nil {
			continue
		} else if err != nil && err != store.ErrMemorySummaryNotFound {
			return counts, err
		}
		if err := b.summaryRepo.Upsert(ctx, buildWeeklySummary(profileID, key, grouped)); err != nil {
			return counts, err
		}
		if b.cacheInvalidator != nil {
			_ = b.cacheInvalidator.OnSummaryChange(ctx, profileID)
		}
		counts.Weekly++
	}

	monthlyGroups := groupNotesByCompletedMonth(notes, now)
	for key, grouped := range monthlyGroups {
		if _, err := b.summaryRepo.GetByPeriod(ctx, profileID, store.SummaryTypeMonthly, key); err == nil {
			continue
		} else if err != nil && err != store.ErrMemorySummaryNotFound {
			return counts, err
		}
		if err := b.summaryRepo.Upsert(ctx, buildMonthlySummary(profileID, key, grouped)); err != nil {
			return counts, err
		}
		if b.cacheInvalidator != nil {
			_ = b.cacheInvalidator.OnSummaryChange(ctx, profileID)
		}
		counts.Monthly++
	}

	return counts, nil
}

// MarkCoveredSummariesStale marks weekly/monthly summaries stale when a note in their covered period changes.
func (b *SummaryRollupBuilder) MarkCoveredSummariesStale(ctx context.Context, profileID string, noteTime time.Time) error {
	if err := b.summaryRepo.MarkFreshness(ctx, profileID, store.SummaryTypeWeekly, WeeklyPeriodKey(noteTime), store.SummaryStale); err != nil {
		return err
	}
	if err := b.summaryRepo.MarkFreshness(ctx, profileID, store.SummaryTypeMonthly, MonthlyPeriodKey(noteTime), store.SummaryStale); err != nil {
		return err
	}
	if b.cacheInvalidator != nil {
		_ = b.cacheInvalidator.OnSummaryChange(ctx, profileID)
	}
	return nil
}

// RegenerateStale rebuilds stale weekly/monthly summaries from the latest covered notes.
func (b *SummaryRollupBuilder) RegenerateStale(ctx context.Context, profileID string, now time.Time) error {
	notes, err := b.noteRepo.ListByProfile(ctx, profileID)
	if err != nil {
		return fmt.Errorf("memory: list notes for regeneration: %w", err)
	}
	weeklyGroups := groupNotesByCompletedWeek(notes, now)
	weekly, err := b.summaryRepo.ListByProfileAndType(ctx, profileID, store.SummaryTypeWeekly)
	if err != nil {
		return err
	}
	for _, s := range weekly {
		if s.FreshnessState != store.SummaryStale {
			continue
		}
		grouped := weeklyGroups[s.PeriodKey]
		if len(grouped) == 0 {
			continue
		}
		rebuilt := buildWeeklySummary(profileID, s.PeriodKey, grouped)
		rebuilt.ID = s.ID
		if err := b.summaryRepo.Upsert(ctx, rebuilt); err != nil {
			return err
		}
		if b.cacheInvalidator != nil {
			_ = b.cacheInvalidator.OnSummaryChange(ctx, profileID)
		}
	}
	monthlyGroups := groupNotesByCompletedMonth(notes, now)
	monthly, err := b.summaryRepo.ListByProfileAndType(ctx, profileID, store.SummaryTypeMonthly)
	if err != nil {
		return err
	}
	for _, s := range monthly {
		if s.FreshnessState != store.SummaryStale {
			continue
		}
		grouped := monthlyGroups[s.PeriodKey]
		if len(grouped) == 0 {
			continue
		}
		rebuilt := buildMonthlySummary(profileID, s.PeriodKey, grouped)
		rebuilt.ID = s.ID
		if err := b.summaryRepo.Upsert(ctx, rebuilt); err != nil {
			return err
		}
		if b.cacheInvalidator != nil {
			_ = b.cacheInvalidator.OnSummaryChange(ctx, profileID)
		}
	}
	return nil
}

func groupNotesByCompletedWeek(notes []*store.MemoryNote, now time.Time) map[string][]*store.MemoryNote {
	cutoff := startOfWeek(now)
	out := map[string][]*store.MemoryNote{}
	for _, n := range notes {
		if !n.CreatedAt.Before(cutoff) {
			continue
		}
		out[WeeklyPeriodKey(n.CreatedAt)] = append(out[WeeklyPeriodKey(n.CreatedAt)], n)
	}
	return out
}

func groupNotesByCompletedMonth(notes []*store.MemoryNote, now time.Time) map[string][]*store.MemoryNote {
	cutoff := firstDayOfMonth(now)
	out := map[string][]*store.MemoryNote{}
	for _, n := range notes {
		if !n.CreatedAt.Before(cutoff) {
			continue
		}
		out[MonthlyPeriodKey(n.CreatedAt)] = append(out[MonthlyPeriodKey(n.CreatedAt)], n)
	}
	return out
}

func buildWeeklySummary(profileID, key string, notes []*store.MemoryNote) *store.MemorySummary {
	periodStart := startOfWeek(notes[0].CreatedAt)
	periodEnd := periodStart.AddDate(0, 0, 7)
	return buildSummary(profileID, store.SummaryTypeWeekly, key, periodStart, periodEnd, notes)
}

func buildMonthlySummary(profileID, key string, notes []*store.MemoryNote) *store.MemorySummary {
	periodStart := firstDayOfMonth(notes[0].CreatedAt)
	periodEnd := periodStart.AddDate(0, 1, 0)
	return buildSummary(profileID, store.SummaryTypeMonthly, key, periodStart, periodEnd, notes)
}

func buildSummary(profileID, summaryType, key string, periodStart, periodEnd time.Time, notes []*store.MemoryNote) *store.MemorySummary {
	sort.Slice(notes, func(i, j int) bool { return notes[i].CreatedAt.Before(notes[j].CreatedAt) })
	ids := make([]string, 0, len(notes))
	parts := make([]string, 0, len(notes))
	for _, n := range notes {
		ids = append(ids, n.ID)
		parts = append(parts, n.Content)
	}
	return &store.MemorySummary{
		ID:                 fmt.Sprintf("%s-%s-%x", summaryType, key, time.Now().UnixNano()),
		ProfileID:          profileID,
		SummaryType:        summaryType,
		PeriodKey:          key,
		PeriodStart:        periodStart,
		PeriodEnd:          periodEnd,
		Content:            summarizeContents(parts),
		SourceStateVersion: SummaryStateVersion(ids),
		FreshnessState:     store.SummaryFresh,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
}

func summarizeContents(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts, " | ")
}

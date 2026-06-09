package memory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"noto/internal/store"
)

// SearchResultItem is a normalized memory search result item.
type SearchResultItem struct {
	RecordType     string
	RecordID       string
	Content        string
	Category       string
	TimeStart      time.Time
	TimeEnd        time.Time
	RelevanceScore *float64
}

// KeywordSearchInput is the input for keyword-based memory search.
type KeywordSearchInput struct {
	Query string
	Limit int
}

// TimeRangeSearchInput is the input for date-bounded memory search.
type TimeRangeSearchInput struct {
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

// KeywordSearchTool executes deterministic keyword-based memory search.
type KeywordSearchTool struct {
	listNotes func(context.Context) ([]*store.MemoryNote, error)
}

// NewKeywordSearchTool creates a keyword memory search tool executor.
func NewKeywordSearchTool(listNotes func(context.Context) ([]*store.MemoryNote, error)) *KeywordSearchTool {
	return &KeywordSearchTool{listNotes: listNotes}
}

// Execute performs keyword-based memory search with deterministic fallback ordering.
func (t *KeywordSearchTool) Execute(ctx context.Context, input KeywordSearchInput) ([]SearchResultItem, error) {
	query := strings.TrimSpace(strings.ToLower(input.Query))
	if query == "" || t == nil || t.listNotes == nil {
		return nil, nil
	}
	notes, err := t.listNotes(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]SearchResultItem, 0)
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n.Content), query) {
			results = append(results, SearchResultItem{
				RecordType: "raw_note",
				RecordID:   n.ID,
				Content:    n.Content,
				Category:   string(n.Category),
				TimeStart:  n.CreatedAt,
				TimeEnd:    n.CreatedAt,
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		// deterministic fallback: importance then recency
		var impI, impJ int
		for _, n := range notes {
			if n.ID == results[i].RecordID {
				impI = n.Importance
			}
			if n.ID == results[j].RecordID {
				impJ = n.Importance
			}
		}
		if impI == impJ {
			return results[i].TimeStart.After(results[j].TimeStart)
		}
		return impI > impJ
	})
	if input.Limit > 0 && len(results) > input.Limit {
		results = results[:input.Limit]
	}
	return results, nil
}

// TimeRangeSearchTool executes time-bounded memory search across notes and summaries.
type TimeRangeSearchTool struct {
	listNotes     func(context.Context) ([]*store.MemoryNote, error)
	listSummaries func(context.Context) ([]*store.MemorySummary, error)
}

// NewTimeRangeSearchTool creates a time-range memory search tool executor.
func NewTimeRangeSearchTool(listNotes func(context.Context) ([]*store.MemoryNote, error), listSummaries func(context.Context) ([]*store.MemorySummary, error)) *TimeRangeSearchTool {
	return &TimeRangeSearchTool{listNotes: listNotes, listSummaries: listSummaries}
}

// Execute performs time-range search and can return mixed raw-note and summary results.
func (t *TimeRangeSearchTool) Execute(ctx context.Context, input TimeRangeSearchInput) ([]SearchResultItem, error) {
	if input.EndTime.Before(input.StartTime) {
		return nil, errors.New("memory: invalid time range")
	}
	results := make([]SearchResultItem, 0)
	if t != nil && t.listNotes != nil {
		notes, err := t.listNotes(ctx)
		if err != nil {
			return nil, err
		}
		for _, n := range notes {
			if !n.CreatedAt.Before(input.StartTime) && !n.CreatedAt.After(input.EndTime) {
				results = append(results, SearchResultItem{
					RecordType: "raw_note",
					RecordID:   n.ID,
					Content:    n.Content,
					Category:   string(n.Category),
					TimeStart:  n.CreatedAt,
					TimeEnd:    n.CreatedAt,
				})
			}
		}
	}
	if t != nil && t.listSummaries != nil {
		summaries, err := t.listSummaries(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range summaries {
			if !s.PeriodStart.After(input.EndTime) && !s.PeriodEnd.Before(input.StartTime) {
				recordType := "weekly_summary"
				if s.SummaryType == store.SummaryTypeMonthly {
					recordType = "monthly_summary"
				}
				results = append(results, SearchResultItem{
					RecordType: recordType,
					RecordID:   s.ID,
					Content:    s.Content,
					TimeStart:  s.PeriodStart,
					TimeEnd:    s.PeriodEnd,
				})
			}
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].TimeStart.Before(results[j].TimeStart) })
	if input.Limit > 0 && len(results) > input.Limit {
		results = results[:input.Limit]
	}
	return results, nil
}

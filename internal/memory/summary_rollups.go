package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

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

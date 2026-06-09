package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"noto/internal/store"
)

func SummaryStateVersion(noteIDs []string) string {
	h := sha256.New()
	for _, id := range noteIDs {
		_, _ = h.Write([]byte(id))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func WeeklyPeriodKey(t time.Time) string {
	start := startOfWeek(t)
	year, week := start.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", year, week)
}

func MonthlyPeriodKey(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%04d-%02d", t.Year(), int(t.Month()))
}

func IsSummaryFresh(summary *store.MemorySummary, stateVersion string) bool {
	if summary == nil {
		return false
	}
	return summary.FreshnessState == store.SummaryFresh && summary.SourceStateVersion == stateVersion
}

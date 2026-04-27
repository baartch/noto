package provider

import "errors"

// Usage captures normalized provider usage fields used by footer telemetry.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	CacheWriteTokens int
	TotalTokens      int
	Cost             float64
	HasUsage         bool
}

// ValidateUsage ensures usage is sane before accumulation.
func ValidateUsage(u Usage) error {
	if u.PromptTokens < 0 || u.CompletionTokens < 0 || u.CachedTokens < 0 || u.CacheWriteTokens < 0 || u.TotalTokens < 0 {
		return errors.New("provider: usage contains negative fields")
	}
	if u.Cost < 0 {
		return errors.New("provider: usage cost is negative")
	}
	return nil
}

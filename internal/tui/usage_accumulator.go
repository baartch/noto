package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"noto/internal/provider"
)

type usageSource int

const (
	usageSourceMain usageSource = iota
	usageSourceExtractor
	usageSourceEmbeddings
)

type usageAccumulator struct {
	up         int
	down       int
	cacheRead  int
	cacheWrite int
	cost       float64
}

func (a *usageAccumulator) addFromUsage(u provider.Usage) {
	if !u.HasUsage {
		return
	}
	a.up += u.PromptTokens
	a.down += u.CompletionTokens
	a.cacheRead += u.CachedTokens
	a.cacheWrite += u.CacheWriteTokens
	a.cost += u.Cost
}

func (a usageAccumulator) formatTokenStatus() string {
	parts := []string{
		"↑" + formatCompactTokens(a.up),
		"↓" + formatCompactTokens(a.down),
	}
	if a.cacheRead > 0 {
		parts = append(parts, "R"+formatCompactTokens(a.cacheRead))
	}
	if a.cacheWrite > 0 {
		parts = append(parts, "W"+formatCompactTokens(a.cacheWrite))
	}
	parts = append(parts, fmt.Sprintf("$%.3f", a.cost))
	return strings.Join(parts, " ")
}

func formatCompactTokens(n int) string {
	if n <= 0 {
		return "0"
	}
	if n >= 1_000_000 {
		v := int(math.Round(float64(n) / 1_000_000))
		if v == 0 {
			v = 1
		}
		return strconv.Itoa(v) + "M"
	}
	if n >= 1_000 {
		v := int(math.Round(float64(n) / 1_000))
		if v == 0 {
			v = 1
		}
		return strconv.Itoa(v) + "k"
	}
	return strconv.Itoa(n)
}

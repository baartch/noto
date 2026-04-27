package tui

import (
	"fmt"

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
	return fmt.Sprintf("↑%d ↓%d cr:%d cw:%d $%.3f", a.up, a.down, a.cacheRead, a.cacheWrite, a.cost)
}

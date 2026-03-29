package provider

import (
	"fmt"
	"strconv"
)

// Stats tracks cumulative token usage and cost for a session.
type Stats struct {
	TokensIn    int     // total prompt tokens sent
	TokensOut   int     // total completion tokens received
	TotalTokens int     // TokensIn + TokensOut
	CostUSD     float64 // estimated cost in USD
	ContextUsed int     // tokens in the most recent request (context window usage)
	ContextMax  int     // context window size (0 = unknown)
}

// Add accumulates stats from a single completion response.
func (s *Stats) Add(resp *CompletionResponse) {
	s.TokensIn += resp.PromptTokens
	s.TokensOut += resp.CompletionTokens
	s.TotalTokens += resp.PromptTokens + resp.CompletionTokens
	s.CostUSD += resp.EstimatedCostUSD
	s.ContextUsed = resp.PromptTokens // last request's prompt size
}

// Format returns a compact status-line string like:
//
//	↑12.4k ↓3.2k  $0.042  68%/200k
func (s Stats) Format() string {
	in := formatTokens(s.TokensIn)
	out := formatTokens(s.TokensOut)
	cost := fmt.Sprintf("$%.3f", s.CostUSD)

	line := fmt.Sprintf("↑%s ↓%s  %s", in, out, cost)
	if s.ContextMax > 0 {
		pct := float64(s.ContextUsed) / float64(s.ContextMax) * 100
		line += fmt.Sprintf("  %.0f%%/%s", pct, formatTokens(s.ContextMax))
	}
	return line
}

func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return strconv.Itoa(n)
	}
}

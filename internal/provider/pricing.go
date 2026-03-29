package provider

import "strings"

// modelPricing holds per-token cost in USD per 1M tokens.
type modelPricing struct {
	inputPer1M    float64
	outputPer1M   float64
	contextWindow int
}

// knownModels is a best-effort pricing and context-window table.
// Prices are USD per 1M tokens as of early 2026 — update as needed.
var knownModels = map[string]modelPricing{
	// OpenAI
	"gpt-4o":        {inputPer1M: 2.50, outputPer1M: 10.00, contextWindow: 128_000},
	"gpt-4o-mini":   {inputPer1M: 0.15, outputPer1M: 0.60, contextWindow: 128_000},
	"gpt-4-turbo":   {inputPer1M: 10.00, outputPer1M: 30.00, contextWindow: 128_000},
	"gpt-4":         {inputPer1M: 30.00, outputPer1M: 60.00, contextWindow: 8_192},
	"gpt-3.5-turbo": {inputPer1M: 0.50, outputPer1M: 1.50, contextWindow: 16_385},
	"o1":            {inputPer1M: 15.00, outputPer1M: 60.00, contextWindow: 200_000},
	"o1-mini":       {inputPer1M: 3.00, outputPer1M: 12.00, contextWindow: 128_000},
	"o3-mini":       {inputPer1M: 1.10, outputPer1M: 4.40, contextWindow: 200_000},
	// Anthropic (via API)
	"claude-3-5-sonnet": {inputPer1M: 3.00, outputPer1M: 15.00, contextWindow: 200_000},
	"claude-sonnet-4-6": {inputPer1M: 3.00, outputPer1M: 15.00, contextWindow: 200_000},
	"claude-3-5-haiku":  {inputPer1M: 0.80, outputPer1M: 4.00, contextWindow: 200_000},
	"claude-3-opus":     {inputPer1M: 15.00, outputPer1M: 75.00, contextWindow: 200_000},
	"claude-3-haiku":    {inputPer1M: 0.25, outputPer1M: 1.25, contextWindow: 200_000},
	// Google
	"gemini-1.5-pro":   {inputPer1M: 3.50, outputPer1M: 10.50, contextWindow: 1_048_576},
	"gemini-1.5-flash": {inputPer1M: 0.075, outputPer1M: 0.30, contextWindow: 1_048_576},
	"gemini-2.0-flash": {inputPer1M: 0.10, outputPer1M: 0.40, contextWindow: 1_048_576},
	// Meta / Ollama (free local — zero cost, variable context)
	"llama3.2": {inputPer1M: 0, outputPer1M: 0, contextWindow: 128_000},
	"llama3.1": {inputPer1M: 0, outputPer1M: 0, contextWindow: 128_000},
	"llama3":   {inputPer1M: 0, outputPer1M: 0, contextWindow: 8_192},
	"mistral":  {inputPer1M: 0, outputPer1M: 0, contextWindow: 32_768},
	"mixtral":  {inputPer1M: 0, outputPer1M: 0, contextWindow: 32_768},
	"phi3":     {inputPer1M: 0, outputPer1M: 0, contextWindow: 128_000},
	"qwen2":    {inputPer1M: 0, outputPer1M: 0, contextWindow: 131_072},
}

// modelInfo looks up pricing/context for a model name with fuzzy prefix matching.
func modelInfo(name string) modelPricing {
	lower := strings.ToLower(name)

	// Exact match first.
	if p, ok := knownModels[lower]; ok {
		return p
	}

	// Prefix/substring match — longest wins.
	best := ""
	var bestP modelPricing
	for key, p := range knownModels {
		if strings.Contains(lower, key) && len(key) > len(best) {
			best = key
			bestP = p
		}
	}
	return bestP
}

// estimateCost returns USD cost for a single completion call.
func estimateCost(model string, promptTokens, completionTokens int) float64 {
	info := modelInfo(model)
	return float64(promptTokens)/1_000_000*info.inputPer1M +
		float64(completionTokens)/1_000_000*info.outputPer1M
}

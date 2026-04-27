package provider_test

import "noto/internal/provider"

func validUsageFixture() provider.Usage {
	return provider.Usage{
		PromptTokens:     194,
		CompletionTokens: 2,
		CachedTokens:     0,
		CacheWriteTokens: 100,
		TotalTokens:      196,
		Cost:             0.95,
		HasUsage:         true,
	}
}

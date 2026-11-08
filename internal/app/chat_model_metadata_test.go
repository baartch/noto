package app

import (
	"testing"

	"noto/internal/provider"
)

func TestResolveSessionModelMetadata(t *testing.T) {
	models := []provider.ModelInfo{
		{ID: "chat", ContextLength: 32_000, ToolSupport: provider.ToolSupport{SupportsTools: true}},
		{ID: "extractor", TopProviderContext: 12_000},
	}
	mainContext, referenceContext, tools, found := resolveSessionModelMetadata(models, "chat", "extractor", false)
	if mainContext != 32_000 || referenceContext != 12_000 || !tools || !found {
		t.Fatalf("explicit extractor metadata = (%d,%d,%t,%t)", mainContext, referenceContext, tools, found)
	}
	mainContext, referenceContext, tools, found = resolveSessionModelMetadata(models, "chat", "chat", true)
	if mainContext != 32_000 || referenceContext != mainContext || !tools || !found {
		t.Fatalf("fallback metadata = (%d,%d,%t,%t)", mainContext, referenceContext, tools, found)
	}
}

func TestResolveExtractorEligibility(t *testing.T) {
	models := []provider.ModelInfo{
		{ID: "chat-ok", SupportsStructuredOutputs: true},
		{ID: "chat-bad"},
		{ID: "ex-ok", SupportsStructuredOutputs: true},
		{ID: "ex-bad"},
	}
	cases := []struct {
		name              string
		active, extractor string
		fallback          bool
		want              bool
	}{
		{"explicit eligible extractor", "chat-ok", "ex-ok", false, true},
		{"explicit ineligible extractor", "chat-ok", "ex-bad", false, false},
		{"fallback eligible chat model", "chat-ok", "chat-ok", true, true},
		{"fallback ineligible chat model", "chat-bad", "chat-bad", true, false},
		{"unknown model id", "chat-ok", "missing", false, false},
		{"empty model list", "chat-ok", "ex-ok", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := models
			if tc.name == "empty model list" {
				list = nil
			}
			if got := resolveExtractorEligibility(list, tc.active, tc.extractor, tc.fallback); got != tc.want {
				t.Fatalf("resolveExtractorEligibility = %t, want %t", got, tc.want)
			}
		})
	}
}

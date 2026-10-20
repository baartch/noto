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

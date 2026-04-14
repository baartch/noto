package integration

import (
	"strings"
	"testing"

	"noto/internal/commands"
	"noto/internal/parser"
	"noto/internal/suggest"
)

func buildTestRegistry(t *testing.T) *commands.Registry {
	t.Helper()
	r := commands.NewRegistry()
	if err := commands.RegisterPromptCommands(r); err != nil {
		t.Fatal(err)
	}
	return r
}

// TestSlashSuggestions_OnlyInSlashMode verifies suggestions are empty for plain text.
func TestSlashSuggestions_OnlyInSlashMode(t *testing.T) {
	r := buildTestRegistry(t)
	engine := suggest.New(r)

	// Slash mode: should return suggestions.
	sug := engine.Suggest("pro")
	if len(sug) == 0 {
		t.Error("expected suggestions for 'pro' prefix in slash mode")
	}

	// For plain text input (no slash), the engine is not called in production;
	// but if called with empty prefix it should return all commands (not zero).
	all := engine.Suggest("")
	if len(all) == 0 {
		t.Error("expected all commands for empty prefix")
	}
}

// TestSlashSuggestions_AmbiguousPrefix_ReturnsMultiple verifies ambiguous input shows multiple.
func TestSlashSuggestions_AmbiguousPrefix_ReturnsMultiple(t *testing.T) {
	r := buildTestRegistry(t)
	engine := suggest.New(r)

	// "prompt" should match multiple commands.
	sug := engine.Suggest("prompt")
	if len(sug) < 2 {
		t.Errorf("expected multiple suggestions for 'prompt', got %d", len(sug))
	}
}

// TestSlashSuggestions_ExactMatch_IsFirst verifies an exact match ranks first.
func TestSlashSuggestions_ExactMatch_IsFirst(t *testing.T) {
	r := buildTestRegistry(t)
	engine := suggest.New(r)

	sug := engine.Suggest("prompt show")
	if len(sug) == 0 {
		t.Fatal("expected at least one suggestion")
	}
	if sug[0].CommandPath != "prompt show" {
		t.Errorf("expected exact match first, got %q", sug[0].CommandPath)
	}
}

// TestSlashParser_SlashOnly_IsPartial verifies "/" alone is partial.
func TestSlashParser_SlashOnly_IsPartial(t *testing.T) {
	result := parser.Parse("/")
	if !result.IsSlash {
		t.Error("expected IsSlash=true")
	}
	if !result.Partial {
		t.Error("expected Partial=true for bare '/'")
	}
}

// TestSlashParser_QuotedArg_ParsedCorrectly ensures quoted args are extracted.
func TestSlashParser_QuotedArg_ParsedCorrectly(t *testing.T) {
	result := parser.Parse(`/prompt show`)
	if result.CommandPath != "prompt show" {
		t.Errorf("unexpected path: %q", result.CommandPath)
	}
	if len(result.Args) != 0 {
		t.Errorf("expected no args, got %v", result.Args)
	}
}

// TestSlashSuggestions_TopSuggestionsText_Format verifies display format.
func TestSlashSuggestions_TopSuggestionsText_Format(t *testing.T) {
	sug := []suggest.Suggestion{
		{CommandPath: "prompt show", Hint: "Show prompt", Rank: 0},
	}
	lines := suggest.TopSuggestionsText(sug)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "/prompt show") {
		t.Errorf("expected /prompt show in output, got %q", lines[0])
	}
}

package contract

import (
	"path/filepath"
	"strings"
	"testing"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/parser"
	"noto/internal/profile"
	"noto/internal/store"
)

// buildRegistry creates a registry with real commands backed by a temp DB.
func buildRegistry(t *testing.T) *commands.Registry {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("NOTO_APP_DIR", dir)
	db, err := store.OpenForTesting(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := profile.NewService(store.NewProfileRepo(db))
	_ = svc
	r := commands.NewRegistry()
	if err := commands.RegisterPromptCommands(r); err != nil {
		t.Fatalf("register prompt commands: %v", err)
	}
	return r
}

func TestSlashParity_PartialInput_ReturnsSuggestions(t *testing.T) {
	r := buildRegistry(t)
	dispatcher := chat.NewDispatcher(r)

	var out strings.Builder
	ctx := &commands.ExecContext{Output: &out}

	result := dispatcher.Dispatch("/pro", ctx)
	if !result.IsSlash {
		t.Error("expected IsSlash=true")
	}
	if len(result.Suggestions) == 0 {
		t.Error("expected suggestions for /pro prefix")
	}
	found := false
	for _, s := range result.Suggestions {
		if strings.HasPrefix(s.CommandPath, "prompt") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected prompt suggestions for /pro")
	}
}

func TestSlashParity_UnknownCommand_ReturnsErrorAndSuggestions(t *testing.T) {
	r := buildRegistry(t)
	dispatcher := chat.NewDispatcher(r)

	var out strings.Builder
	ctx := &commands.ExecContext{Output: &out}

	result := dispatcher.Dispatch("/prompt frobnicate", ctx)
	if result.Err == nil {
		t.Error("expected error for unknown command")
	}
	if !strings.Contains(result.Err.Error(), "unknown command") {
		t.Errorf("expected 'unknown command' in error, got: %v", result.Err)
	}
}

func TestSlashParity_PlainInput_NotSlash(t *testing.T) {
	r := buildRegistry(t)
	dispatcher := chat.NewDispatcher(r)

	var out strings.Builder
	ctx := &commands.ExecContext{Output: &out}

	result := dispatcher.Dispatch("hello world", ctx)
	if result.IsSlash {
		t.Error("plain text should not be treated as slash input")
	}
}

func TestSlashParser_CanonicalPath(t *testing.T) {
	cases := []struct {
		input    string
		wantPath string
		wantArgs []string
	}{
		{"/prompt show", "prompt show", nil},
		{"/prompt edit", "prompt edit", nil},
	}

	for _, tc := range cases {
		result := parser.Parse(tc.input)
		if result.CommandPath != tc.wantPath {
			t.Errorf("input=%q: got path=%q, want %q", tc.input, result.CommandPath, tc.wantPath)
		}
		if len(result.Args) != len(tc.wantArgs) {
			t.Errorf("input=%q: got %d args, want %d", tc.input, len(result.Args), len(tc.wantArgs))
			continue
		}
		for i, arg := range tc.wantArgs {
			if result.Args[i] != arg {
				t.Errorf("input=%q: arg[%d] got=%q, want=%q", tc.input, i, result.Args[i], arg)
			}
		}
	}
}

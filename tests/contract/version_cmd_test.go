package contract

import (
	"bytes"
	"strings"
	"testing"

	"noto/internal/app"
	"noto/internal/version"
)

func TestVersionCommand_PrintsVersion(t *testing.T) {
	old := version.Version
	version.Version = "v1.2.3"
	t.Cleanup(func() { version.Version = old })

	cmd := app.RootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "v1.2.3" {
		t.Fatalf("expected v1.2.3, got %q", got)
	}
}

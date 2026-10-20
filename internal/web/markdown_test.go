package web

import (
	"strings"
	"testing"
)

func TestRenderMarkdownSanitizesExecutableContentAndPreservesFormatting(t *testing.T) {
	got := RenderMarkdown("<script>alert(1)</script>\n\n[bad](javascript:alert(1))\n\n**bold**\n\n```go\nfmt.Println(1)\n```")
	for _, unsafe := range []string{"<script>", "href=\"javascript:", "javascript:"} {
		if strings.Contains(strings.ToLower(got), unsafe) {
			t.Fatalf("unsafe markdown output contains %q: %s", unsafe, got)
		}
	}
	for _, safe := range []string{"<strong>bold</strong>", "<pre>", "fmt.Println(1)"} {
		if !strings.Contains(got, safe) {
			t.Fatalf("sanitized markdown omits %q: %s", safe, got)
		}
	}
}

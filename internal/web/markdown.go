package web

import (
	"bytes"
	"html"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	markdownRenderer = goldmark.New(goldmark.WithExtensions(extension.GFM))
	markdownPolicy   = bluemonday.UGCPolicy()
)

// RenderMarkdown renders user/provider text and removes unsafe HTML and URLs.
func RenderMarkdown(source string) string {
	var rendered bytes.Buffer
	if err := markdownRenderer.Convert([]byte(source), &rendered); err != nil {
		return html.EscapeString(source)
	}
	return markdownPolicy.Sanitize(rendered.String())
}

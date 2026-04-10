package tui

import "testing"

func TestStylesRenderNonEmpty(t *testing.T) {
	cases := []struct {
		name  string
		style func() string
	}{
		{"header", func() string { return headerStyle.Render("x") }},
		{"error", func() string { return errStyle.Render("x") }},
		{"suggestion", func() string { return suggNormalStyle.Render("x") }},
		{"picker", func() string { return pickerNormalStyle.Render("x") }},
	}

	for _, tt := range cases {
		if got := tt.style(); got == "" {
			t.Fatalf("expected %s style to render output", tt.name)
		}
	}
}

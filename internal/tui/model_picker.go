package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// pickerState is a sub-view overlaid on the chat when /model is invoked.
type pickerState struct {
	models  []string // model IDs
	cursor  int
	filter  string // live filter string typed by the user
	loading bool
	err     error
}

var (
	pickerBorderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)
	pickerCursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	pickerNormalStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	pickerFilterStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	pickerHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)
)

// filtered returns the subset of models matching the current filter string.
func (p *pickerState) filtered() []string {
	if p.filter == "" {
		return p.models
	}
	f := strings.ToLower(p.filter)
	var out []string
	for _, m := range p.models {
		if strings.Contains(strings.ToLower(m), f) {
			out = append(out, m)
		}
	}
	return out
}

// clampCursor keeps the cursor within bounds of the filtered list.
func (p *pickerState) clampCursor() {
	list := p.filtered()
	if p.cursor < 0 {
		p.cursor = 0
	}
	if len(list) > 0 && p.cursor >= len(list) {
		p.cursor = len(list) - 1
	}
}

// selected returns the currently highlighted model ID, or "" if none.
func (p *pickerState) selected() string {
	list := p.filtered()
	if len(list) == 0 || p.cursor >= len(list) {
		return ""
	}
	return list[p.cursor]
}

// render draws the picker as a string to be shown above the input line.
func (p *pickerState) render(maxHeight int) string {
	if p.loading {
		return pickerBorderStyle.Render(pickerHeaderStyle.Render("  Fetching models…"))
	}
	if p.err != nil {
		return pickerBorderStyle.Render(errStyle.Render("  Error: " + p.err.Error()))
	}

	list := p.filtered()
	var sb strings.Builder
	sb.WriteString(pickerHeaderStyle.Render("  Select model  (↑↓ navigate · Enter select · Esc cancel)") + "\n")
	sb.WriteString(pickerFilterStyle.Render(fmt.Sprintf("  Filter: %s▌\n", p.filter)))

	// Show at most maxHeight-3 rows (header + filter + border).
	maxRows := maxHeight - 4
	if maxRows < 3 {
		maxRows = 3
	}

	// Determine visible window around cursor.
	start := p.cursor - maxRows/2
	if start < 0 {
		start = 0
	}
	end := start + maxRows
	if end > len(list) {
		end = len(list)
		start = end - maxRows
		if start < 0 {
			start = 0
		}
	}

	if len(list) == 0 {
		sb.WriteString(pickerNormalStyle.Render("  (no models match)"))
	}
	for i := start; i < end; i++ {
		prefix := "   "
		style := pickerNormalStyle
		if i == p.cursor {
			prefix = " › "
			style = pickerCursorStyle
		}
		sb.WriteString(style.Render(prefix+list[i]) + "\n")
	}

	if len(list) > maxRows {
		sb.WriteString(pickerNormalStyle.Render(fmt.Sprintf("  … %d models total", len(list))))
	}

	return pickerBorderStyle.Render(sb.String())
}

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	pickerBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)
	pickerCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	pickerNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	pickerActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // currently-active item
	pickerFilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	pickerHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)
)

// pickerItem is a single entry in the picker list.
type pickerItem struct {
	// Value is the ID/slug passed to the selected callback.
	Value string
	// Label is the display string shown in the list (defaults to Value if empty).
	Label string
	// Active marks the currently selected/active item (shown with a different style).
	Active bool
}

func (it pickerItem) display() string {
	if it.Label != "" {
		return it.Label
	}
	return it.Value
}

// pickerState is the generic overlay picker used for model and profile selection.
type pickerState struct {
	title   string
	items   []pickerItem
	cursor  int
	filter  string
	loading bool
	err     error
}

// filtered returns items whose display string contains the filter (case-insensitive).
func (p *pickerState) filtered() []pickerItem {
	if p.filter == "" {
		return p.items
	}
	f := strings.ToLower(p.filter)
	var out []pickerItem
	for _, it := range p.items {
		if strings.Contains(strings.ToLower(it.display()), f) {
			out = append(out, it)
		}
	}
	return out
}

// clampCursor keeps cursor within the filtered list bounds.
func (p *pickerState) clampCursor() {
	list := p.filtered()
	if p.cursor < 0 {
		p.cursor = 0
	}
	if len(list) > 0 && p.cursor >= len(list) {
		p.cursor = len(list) - 1
	}
}

// selectedValue returns the Value of the highlighted item, or "".
func (p *pickerState) selectedValue() string {
	list := p.filtered()
	if len(list) == 0 || p.cursor >= len(list) {
		return ""
	}
	return list[p.cursor].Value
}

// render draws the picker, fitting within maxHeight terminal rows.
func (p *pickerState) render(maxHeight int) string {
	if p.loading {
		return pickerBorderStyle.Render(pickerHeaderStyle.Render("  Loading…"))
	}
	if p.err != nil {
		return pickerBorderStyle.Render(errStyle.Render("  Error: " + p.err.Error()))
	}

	list := p.filtered()
	var sb strings.Builder
	sb.WriteString(pickerHeaderStyle.Render("  "+p.title+"  (↑↓ navigate · type to filter · Enter select · Esc cancel)") + "\n")
	sb.WriteString(pickerFilterStyle.Render(fmt.Sprintf("  Filter: %s▌\n", p.filter)))

	maxRows := maxHeight - 4
	if maxRows < 3 {
		maxRows = 3
	}

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
		sb.WriteString(pickerNormalStyle.Render("  (no matches)"))
	}
	for i := start; i < end; i++ {
		it := list[i]
		prefix := "   "
		var style lipgloss.Style
		switch {
		case i == p.cursor:
			prefix = " › "
			style = pickerCursorStyle
		case it.Active:
			prefix = " ● "
			style = pickerActiveStyle
		default:
			style = pickerNormalStyle
		}
		sb.WriteString(style.Render(prefix+it.display()) + "\n")
	}
	if len(list) > maxRows {
		sb.WriteString(pickerNormalStyle.Render(fmt.Sprintf("  … %d items", len(list))))
	}

	return pickerBorderStyle.Render(sb.String())
}

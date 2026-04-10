package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func (it pickerItem) Title() string       { return it.display() }
func (it pickerItem) Description() string { return "" }
func (it pickerItem) FilterValue() string { return it.display() }

// pickerDelegate renders picker items with active/cursor markers.
type pickerDelegate struct{}

func (d pickerDelegate) Height() int  { return 1 }
func (d pickerDelegate) Spacing() int { return 0 }
func (d pickerDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d pickerDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(pickerItem)
	if !ok {
		return
	}
	indicator := " "
	style := pickerNormalStyle
	switch {
	case index == m.Index():
		indicator = "›"
		style = pickerCursorStyle
	case it.Active:
		indicator = "●"
		style = pickerActiveStyle
	}

	line := "  " + indicator + " " + it.display()
	fmt.Fprint(w, style.Render(fitLine(line, m.Width()))+"\n")
}

// pickerState is the generic overlay picker used for model and profile selection.
type pickerState struct {
	title   string
	list    list.Model
	loading bool
	err     error
	width   int
}

func newPickerState(title string, width int) *pickerState {
	delegate := pickerDelegate{}
	l := list.New([]list.Item{}, delegate, width, 0)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowFilter(true)
	l.Title = "  " + title + "  (↑↓ navigate · type to filter · Enter select · Esc cancel)"
	return &pickerState{title: title, list: l, width: width}
}

func (p *pickerState) setItems(items []pickerItem) {
	listItems := make([]list.Item, len(items))
	for i, it := range items {
		listItems[i] = it
	}
	p.list.SetItems(listItems)
	p.list.Select(0)
}

// selectedValue returns the Value of the highlighted item, or "".
func (p *pickerState) selectedValue() string {
	item := p.list.SelectedItem()
	if item == nil {
		return ""
	}
	return item.(pickerItem).Value
}

// render draws the picker, fitting within maxHeight terminal rows.
func (p *pickerState) render(maxHeight int) string {
	width := p.width
	if width <= 0 {
		width = 50
	}
	if p.loading {
		return pickerBorderStyle.Render(pickerHeaderStyle.Render(fitLine("  Loading…", width)))
	}
	if p.err != nil {
		return pickerBorderStyle.Render(errStyle.Render(fitLine("  Error: "+p.err.Error(), width)))
	}

	maxRows := maxHeight - 2
	maxRows = max(maxRows, 5)
	p.list.SetSize(width-2, maxRows)

	return pickerBorderStyle.Render(p.list.View())
}

func fitLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	pad := width - lipgloss.Width(line)
	if pad <= 0 {
		return line
	}
	return line + strings.Repeat(" ", pad)
}

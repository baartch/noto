package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
)

// ---- palette ----------------------------------------------------------------

var (
	// User bubble
	userBubbleBg   = lipgloss.Color("25")  // dark blue
	userBubbleFg   = lipgloss.Color("255") // white
	userLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	userTimeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Assistant bubble
	asstLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Bold(true)
	asstTimeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Command output
	cmdLineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
	cmdPrefixStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Assistant markdown block
	assistantBorderStyle = lipgloss.NewStyle().
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(lipgloss.Color("63")).
				PaddingLeft(1)
	modelLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Common UI styles
	headerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(false)
	errStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	suggNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	suggSelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63")).Bold(true)
	dividerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	promptStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	cursorStyleDef  = textarea.CursorStyle{Color: lipgloss.Color("63"), Shape: tea.CursorBlock, Blink: true}

	// Picker styles
	pickerBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)
	pickerCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	pickerNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	pickerActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // currently-active item
	pickerFilterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	pickerHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)
)

package tui

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ---- palette ----------------------------------------------------------------

var (
	helpShortStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpFullStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

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
	boundaryStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	promptStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyleDef  = textarea.CursorStyle{Color: lipgloss.Color("63"), Shape: tea.CursorBlock, Blink: true}

	// Picker styles
	pickerBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1)
	pickerCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	pickerNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	pickerActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // currently-active item
	pickerHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)

	// Footer status styles
	footerDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	footerGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("71"))
	footerBlueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	footerYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	footerPurpleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	footerWhiteStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// Sidebar styles
	sidebarBorder      = lipgloss.NewStyle().BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("39")).PaddingLeft(1)
	sidebarActiveTab   = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Underline(true)
	sidebarInactiveTab = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sidebarEntryBorder = lipgloss.NewStyle().Background(lipgloss.Color("233")).Foreground(lipgloss.Color("252")).Padding(0, 1)
	sidebarMetaStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sidebarLoading     = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	sidebarEmpty       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Select mode styles
	selectBgStyle = lipgloss.NewStyle().Background(lipgloss.Color("17")).Foreground(lipgloss.Color("252"))
)

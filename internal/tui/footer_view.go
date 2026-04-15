package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// renderFooter draws the bottom status line.
func (m *Model) renderFooter() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("71"))
	blue := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	purple := lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	white := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// Left: token stats + cache status + notes badge + extractor warning.
	var leftParts []string

	if m.tokenStatus != "" {
		leftParts = append(leftParts, blue.Render(m.tokenStatus))
	}

	cache := strings.TrimSpace(m.cacheStatus)
	switch {
	case strings.Contains(cache, "hit"):
		leftParts = append(leftParts, green.Render("ctx:hit"))
	case strings.Contains(cache, "miss"):
		leftParts = append(leftParts, yellow.Render("ctx:miss"))
	default:
		leftParts = append(leftParts, dim.Render("ctx:n/a"))
	}

	if m.notesIndicator != "" {
		leftParts = append(leftParts, green.Render(m.notesIndicator))
	}
	if m.extractorFallback {
		leftParts = append(leftParts, yellow.Render("Extractor model missing — using main model."))
	}
	if m.embeddingModelMissing {
		leftParts = append(leftParts, yellow.Render("Embeddings model missing — memory disabled."))
	}

	left := strings.Join(leftParts, dim.Render("  "))

	// Right: profile + model + help.
	right := white.Render(m.profileName)
	if m.activeModel != "" {
		right = right + dim.Render("  ") + purple.Render("["+m.activeModel+"]")
	}
	helpView := m.help.ShortHelpView(m.helpKeys.ShortHelp())
	if helpView != "" {
		right = right + dim.Render("  ") + helpView
	}

	_ = yellow // suppress unused if no cost yet

	margin := lipgloss.Width(m.input.Prompt) + 1
	margin = max(margin, 0)
	innerWidth := m.width - margin*2
	innerWidth = max(innerWidth, 0)
	pad := strings.Repeat(" ", margin)
	return pad + footerLine(innerWidth, left, right) + pad
}

// footerLine pads left/right content to terminal width.
func footerLine(width int, left, right string) string {
	if width <= 0 {
		return left + "  " + right
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left + "  " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

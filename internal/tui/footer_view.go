package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"noto/internal/version"
)

// renderFooter draws the bottom status line.
func (m *Model) renderFooter() string {
	dim := footerDimStyle
	green := footerGreenStyle
	blue := footerBlueStyle
	yellow := footerYellowStyle
	purple := footerPurpleStyle
	white := footerWhiteStyle

	// Left: token stats + cache status + notes badge + extractor warning.
	var leftParts []string

	if m.tokenStatus != "" {
		leftParts = append(leftParts, blue.Render(m.tokenStatus))
	}

	cache := strings.TrimSpace(m.cacheStatus)
	switch {
	case strings.Contains(cache, "ctx:l1-hit") || strings.Contains(cache, "ctx:l2-hit") || strings.Contains(cache, "ctx:hit"):
		leftParts = append(leftParts, green.Render(cache))
	case strings.Contains(cache, "ctx:swr"):
		leftParts = append(leftParts, blue.Render(cache))
	case strings.Contains(cache, "ctx:rebuild"):
		leftParts = append(leftParts, green.Render(cache))
	case strings.Contains(cache, "ctx:miss") || strings.Contains(cache, "ctx:error"):
		leftParts = append(leftParts, yellow.Render(cache))
	default:
		leftParts = append(leftParts, dim.Render("ctx:n/a"))
	}

	if m.notesIndicator != "" {
		leftParts = append(leftParts, green.Render(m.notesIndicator))
	}
	if m.updateNotice != "" {
		leftParts = append(leftParts, yellow.Render(m.updateNotice))
	}
	if m.extractorFallback {
		leftParts = append(leftParts, yellow.Render("Extractor model missing — using main model."))
	}
	if m.embeddingModelMissing {
		leftParts = append(leftParts, yellow.Render("Embeddings model missing — memory disabled."))
	}
	if m.promptBootstrapWarning {
		leftParts = append(leftParts, yellow.Render("Prompt files missing — bootstrapped defaults."))
	}

	left := strings.Join(leftParts, dim.Render("  "))

	// Right: profile + model + version + help.
	right := white.Render(m.profileName)
	if m.activeModel != "" {
		right = right + dim.Render("  ") + purple.Render("["+m.activeModel+"]")
	}
	right = right + dim.Render("  ") + dim.Render(version.String())
	helpView := m.help.ShortHelpView(m.helpKeys.ShortHelp())
	if helpView != "" {
		right = right + dim.Render("  ") + helpView
	}

	_ = yellow // suppress unused if no cost yet

	margin := lipgloss.Width(m.input.Prompt) + 1
	margin = max(margin, 0)
	innerWidth := m.width - margin*2
	if m.sidebar != nil && m.sidebar.open {
		innerWidth -= m.sidebar.width + 1
	}
	innerWidth = max(innerWidth, 0)
	pad := strings.Repeat(" ", margin)
	return pad + footerLine(innerWidth, left, right) + pad
}

// footerLine pads left/right content to terminal width.
func footerLine(width int, left, right string) string {
	if width <= 0 {
		return left + "  " + right
	}

	rightWidth := lipgloss.Width(right)
	if rightWidth >= width {
		return right
	}

	maxLeft := max(width-rightWidth-2, 0)
	if lipgloss.Width(left) > maxLeft {
		left = lipgloss.NewStyle().MaxWidth(maxLeft).Render(left)
	}

	gap := max(width-lipgloss.Width(left)-rightWidth, 2)
	return left + strings.Repeat(" ", gap) + right
}

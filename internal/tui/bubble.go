package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ---- palette ----------------------------------------------------------------

var (
	// User bubble
	userBubbleBg   = lipgloss.Color("25")  // dark blue
	userBubbleFg   = lipgloss.Color("255") // white
	userLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	userTimeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Assistant bubble
	asstBubbleBg   = lipgloss.Color("235") // dark grey
	asstBubbleFg   = lipgloss.Color("252")
	asstLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Bold(true)
	asstTimeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Command output
	cmdLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
	cmdPrefixStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// renderer is a cached glamour renderer — recreated when width changes.
type mdRenderer struct {
	r     *glamour.TermRenderer
	width int
}

var cachedRenderer mdRenderer

func renderMarkdown(content string, maxWidth int) string {
	// Clamp to a readable width.
	w := maxWidth - 6 // subtract bubble padding
	if w < 40 {
		w = 40
	}
	if w > 120 {
		w = 120
	}

	if cachedRenderer.r == nil || cachedRenderer.width != w {
		// Use an explicit dark style instead of WithAutoStyle().
		// WithAutoStyle() queries the terminal background via OSC ]11;?
		// which causes the response sequence to appear in the text input.
		r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(w),
		)
		if err != nil {
			return content
		}
		cachedRenderer = mdRenderer{r: r, width: w}
	}

	out, err := cachedRenderer.r.Render(content)
	if err != nil {
		return content
	}
	// glamour adds a trailing newline — trim to one.
	return strings.TrimRight(out, "\n")
}

// renderUserBubble renders a right-aligned user message bubble.
func renderUserBubble(content, authorName string, ts time.Time, termWidth int) string {
	maxW := termWidth - 4
	if maxW > 80 {
		maxW = 80
	}

	bubble := lipgloss.NewStyle().
		Background(userBubbleBg).
		Foreground(userBubbleFg).
		Padding(0, 2).
		Width(maxW).
		Render(wordWrap(content, maxW-4))

	label := userLabelStyle.Render(authorName) +
		"  " + userTimeStyle.Render(ts.Format("15:04"))

	// Right-align both label and bubble.
	rightPad := termWidth - maxW - 2
	if rightPad < 0 {
		rightPad = 0
	}
	pad := strings.Repeat(" ", rightPad)

	return pad + label + "\n" + pad + bubble
}

// renderAssistantBubble renders a left-aligned assistant message with markdown.
func renderAssistantBubble(content, modelName string, ts time.Time, termWidth int) string {
	rendered := renderMarkdown(content, termWidth)

	// Wrap rendered markdown in a subtle left border.
	lines := strings.Split(rendered, "\n")
	var sb strings.Builder
	borderStyle := lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(lipgloss.Color("63")).
		PaddingLeft(1)

	inner := strings.Join(lines, "\n")
	boxed := borderStyle.Render(inner)

	modelLabel := ""
	if modelName != "" {
		modelLabel = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("["+modelName+"]")
	}
	label := asstLabelStyle.Render("Noto") +
		modelLabel +
		"  " + asstTimeStyle.Render(ts.Format("15:04"))

	sb.WriteString(label + "\n")
	sb.WriteString(boxed)
	return sb.String()
}

// renderCommandLine renders inline command output (dimmed, no bubble).
func renderCommandLine(content string) string {
	var sb strings.Builder
	for _, line := range strings.Split(content, "\n") {
		sb.WriteString(cmdPrefixStyle.Render("  ❯ ") + cmdLineStyle.Render(line) + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// wordWrap wraps text at maxWidth characters, respecting word boundaries.
func wordWrap(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var current strings.Builder
	lineLen := 0

	for _, word := range words {
		wLen := utf8.RuneCountInString(word)
		if lineLen == 0 {
			current.WriteString(word)
			lineLen = wLen
		} else if lineLen+1+wLen <= maxWidth {
			current.WriteByte(' ')
			current.WriteString(word)
			lineLen += 1 + wLen
		} else {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
			lineLen = wLen
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return strings.Join(lines, "\n")
}

// formatTimestamp returns a display timestamp for a message.
// Shows time today, or "Mon 15:04" for older messages.
func formatTimestamp(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return fmt.Sprintf("%s %s", t.Weekday().String()[:3], t.Format("15:04"))
}

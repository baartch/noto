package tui

import (
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
	asstLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Bold(true)
	asstTimeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Command output
	cmdLineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
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
	w = max(w, 40)
	w = min(w, 120)

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
	if termWidth < 20 {
		termWidth = 80
	}

	// Bubble occupies at most 70% of terminal width, minimum 40 cols.
	bubbleW := int(float64(termWidth) * 0.70)
	bubbleW = max(bubbleW, 40)
	bubbleW = min(bubbleW, termWidth-2)
	innerW := bubbleW - 4 // subtract horizontal padding (2 each side)

	wrapped := wordWrap(content, innerW)
	bubble := lipgloss.NewStyle().
		Background(userBubbleBg).
		Foreground(userBubbleFg).
		Padding(0, 2).
		Width(bubbleW).
		Render(wrapped)

	label := userLabelStyle.Render(authorName) +
		"  " + userTimeStyle.Render(ts.Format("15:04"))

	// Right-align: pad = space to push bubble to the right edge.
	leftPad := termWidth - bubbleW
	leftPad = max(leftPad, 0)
	pad := strings.Repeat(" ", leftPad)

	paddedBubble := padLines(bubble, pad)
	return pad + label + "\n" + paddedBubble
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

	// Indent wrapped lines to align with the first line.
	inner := alignWrappedLines(lines, "  ")
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
	for line := range strings.SplitSeq(content, "\n") {
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
		switch {
		case lineLen == 0:
			current.WriteString(word)
			lineLen = wLen
		case lineLen+1+wLen <= maxWidth:
			current.WriteByte(' ')
			current.WriteString(word)
			lineLen += 1 + wLen
		default:
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
func padLines(content, pad string) string {
	if pad == "" {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

func alignWrappedLines(lines []string, indent string) string {
	var out []string
	for i, line := range lines {
		if i == 0 {
			out = append(out, line)
			continue
		}
		if strings.TrimSpace(line) == "" {
			out = append(out, line)
			continue
		}
		out = append(out, indent+line)
	}
	return strings.Join(out, "\n")
}

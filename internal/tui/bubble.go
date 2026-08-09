package tui

import (
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
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
func renderUserBubble(content, authorName string, ts time.Time, termWidth int, selected bool) string {
	if termWidth < 20 {
		termWidth = 80
	}

	// Bubble occupies at most 70% of terminal width, minimum 40 cols.
	bubbleW := int(float64(termWidth) * 0.70)
	bubbleW = max(bubbleW, 40)
	bubbleW = min(bubbleW, termWidth-2)
	innerW := bubbleW - 4 // subtract horizontal padding (2 each side)

	bubbleBg := userBubbleBg
	bubbleFg := userBubbleFg
	if selected {
		bubbleBg = lipgloss.Color("18")
		bubbleFg = lipgloss.Color("255")
	}

	wrapped := wordWrap(content, innerW)
	bubble := lipgloss.NewStyle().
		Background(bubbleBg).
		Foreground(bubbleFg).
		Padding(0, 2).
		Width(bubbleW).
		Render(wrapped)

	labelStyle := userLabelStyle
	if selected {
		labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	}
	name := authorName
	if selected {
		name = "► " + authorName
	}
	label := labelStyle.Render(name) +
		"  " + userTimeStyle.Render(ts.Format("15:04"))

	// Right-align: pad = space to push bubble to the right edge.
	leftPad := termWidth - bubbleW
	leftPad = max(leftPad, 0)
	pad := strings.Repeat(" ", leftPad)

	paddedBubble := padLines(bubble, pad)
	return pad + label + "\n" + paddedBubble
}

// renderAssistantBubble renders a left-aligned assistant message with markdown.
func renderAssistantBubble(content, modelName string, ts time.Time, termWidth int, selected bool) string {
	rendered := renderMarkdown(content, termWidth)

	// Wrap rendered markdown in a subtle left border.
	lines := strings.Split(rendered, "\n")
	var sb strings.Builder
	borderStyle := assistantBorderStyle
	if selected {
		borderStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("18")). // invisible on same-color bg, preserves geometry
			PaddingLeft(1).
			Background(lipgloss.Color("18")).
			Foreground(lipgloss.Color("255")).
			Width(max(termWidth-4, 1))
	}

	// Indent wrapped lines to align with the first line.
	inner := alignWrappedLines(lines, "  ")
	boxed := borderStyle.Render(inner)

	marker := ""
	if selected {
		marker = "► "
	}
	modelLabel := ""
	if modelName != "" {
		modelLabel = "  " + modelLabelStyle.Render("["+modelName+"]")
	}
	label := asstLabelStyle.Render(marker+"Noto") +
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

// wordWrap wraps text at maxWidth characters, preserving explicit line breaks
// and indentation while still wrapping long lines at word boundaries.
func wordWrap(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	lines := strings.Split(text, "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, wrapLinePreservingIndent(line, maxWidth)...)
	}
	return strings.Join(wrapped, "\n")
}

func wrapLinePreservingIndent(line string, maxWidth int) []string {
	if line == "" {
		return []string{""}
	}

	indentWidth := 0
	indentByteEnd := 0
	for i, r := range line {
		if r != ' ' && r != '\t' {
			break
		}
		indentWidth++
		indentByteEnd = i + utf8.RuneLen(r)
	}
	indent := line[:indentByteEnd]
	rest := line[indentByteEnd:]
	if rest == "" {
		return []string{line}
	}

	words := strings.FieldsFunc(rest, func(r rune) bool {
		return r == ' '
	})
	if len(words) == 0 {
		return []string{line}
	}

	var out []string
	var current strings.Builder
	current.WriteString(indent)
	lineLen := indentWidth

	for _, word := range words {
		wLen := utf8.RuneCountInString(word)
		switch {
		case lineLen == indentWidth:
			current.WriteString(word)
			lineLen += wLen
		case lineLen+1+wLen <= maxWidth:
			current.WriteByte(' ')
			current.WriteString(word)
			lineLen += 1 + wLen
		default:
			out = append(out, current.String())
			current.Reset()
			current.WriteString(indent)
			current.WriteString(word)
			lineLen = indentWidth + wLen
		}
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
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

func renderConversationBoundary(ts time.Time, width int) string {
	if width < 20 {
		width = 20
	}
	label := " " + ts.In(time.Local).Format("2006-01-02 15:04 MST") + " "
	labelW := utf8.RuneCountInString(label)
	lineRun := "─"

	remaining := max(width-labelW, 4)
	left := remaining / 2
	right := remaining - left

	line := strings.Repeat(lineRun, left) + label + strings.Repeat(lineRun, right)
	return boundaryStyle.Render(line)
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

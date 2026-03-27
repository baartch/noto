package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/suggest"
)

// ProviderFunc is the function signature for sending a chat turn to a provider.
// It receives the user message and returns the assistant reply or an error.
type ProviderFunc func(ctx context.Context, userMsg string) (string, error)

// Model is the root Bubble Tea model for Noto.
type Model struct {
	profileName string
	input       textinput.Model
	viewport    viewport.Model
	messages    []chatMessage
	suggestions []suggest.Suggestion
	width       int
	height      int
	err         error
	ready       bool

	dispatcher  *chat.Dispatcher
	execCtx     *commands.ExecContext
	provider    ProviderFunc // nil = no provider configured
}

type chatMessage struct {
	role    string
	content string
}

// providerReplyMsg carries an async provider response back to the Update loop.
type providerReplyMsg struct {
	content string
	err     error
}

// Styles.
var (
	headerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	suggStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	cmdOutStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
)

// New creates a new TUI Model.
func New(
	profileName string,
	dispatcher *chat.Dispatcher,
	execCtx *commands.ExecContext,
	provider ProviderFunc,
) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message or /command…"
	ti.Focus()
	ti.CharLimit = 4096

	return Model{
		profileName: profileName,
		input:       ti,
		dispatcher:  dispatcher,
		execCtx:     execCtx,
		provider:    provider,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4

		headerHeight := 3
		inputHeight := 3
		vpHeight := msg.Height - headerHeight - inputHeight
		if vpHeight < 1 {
			vpHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(msg.Width, vpHeight)
			m.viewport.SetContent(m.renderHistory())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vpHeight
		}

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.suggestions = nil
			m.err = nil

			// Try slash dispatch first.
			result := m.dispatcher.Dispatch(val, m.execCtx)

			if result.IsSlash {
				if result.Err != nil {
					m.err = result.Err
				} else if result.Executed && result.Output != "" {
					m.messages = append(m.messages, chatMessage{
						role:    "command",
						content: strings.TrimRight(result.Output, "\n"),
					})
				}
				m.syncViewport()
				return m, nil
			}

			// Plain chat — add user message and call provider async.
			m.messages = append(m.messages, chatMessage{role: "user", content: val})
			m.syncViewport()

			if m.provider == nil {
				m.messages = append(m.messages, chatMessage{
					role:    "assistant",
					content: "No provider configured. Set NOTO_API_KEY and NOTO_MODEL env vars, or use `noto provider set`.",
				})
				m.syncViewport()
				return m, nil
			}

			userVal := val
			cmds = append(cmds, func() tea.Msg {
				reply, err := m.provider(context.Background(), userVal)
				return providerReplyMsg{content: reply, err: err}
			})

		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			cmds = append(cmds, vpCmd)
			return m, tea.Batch(cmds...)
		}

	case providerReplyMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.messages = append(m.messages, chatMessage{role: "assistant", content: msg.content})
		}
		m.syncViewport()
		return m, nil
	}

	// Forward to input and refresh suggestions.
	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	cmds = append(cmds, inputCmd)

	val := m.input.Value()
	if strings.HasPrefix(val, "/") {
		prefix := val[1:]
		if m.dispatcher != nil {
			// Re-use engine via partial dispatch.
			result := m.dispatcher.Dispatch(val+" ", m.execCtx) // trailing space → partial
			m.suggestions = result.Suggestions
		} else {
			_ = prefix
		}
	} else {
		m.suggestions = nil
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "\n  Initialising…"
	}

	header := headerStyle.Render("─── Noto · " + m.profileName + " ── Ctrl+C to quit ───")

	// Suggestions overlay (shown above input).
	var suggBlock string
	if len(m.suggestions) > 0 {
		var sb strings.Builder
		for _, s := range m.suggestions {
			sb.WriteString(suggStyle.Render("  /"+s.CommandPath+"  — "+s.Hint) + "\n")
		}
		suggBlock = sb.String()
	}

	var errBlock string
	if m.err != nil {
		errBlock = errStyle.Render("  ✗ "+m.err.Error()) + "\n"
	}

	return header + "\n" +
		m.viewport.View() + "\n" +
		suggBlock +
		errBlock +
		m.input.View()
}

// syncViewport rebuilds the viewport content and scrolls to the bottom.
func (m *Model) syncViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
}

// renderHistory renders all chat messages into a single string.
func (m *Model) renderHistory() string {
	if len(m.messages) == 0 {
		return headerStyle.Render("  No messages yet. Start typing below.")
	}
	var sb strings.Builder
	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			sb.WriteString(userStyle.Render("You:   ") + msg.content + "\n\n")
		case "assistant":
			sb.WriteString(assistantStyle.Render("Noto:  ") + msg.content + "\n\n")
		case "command":
			sb.WriteString(cmdOutStyle.Render(msg.content) + "\n\n")
		}
	}
	return sb.String()
}

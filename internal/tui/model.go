package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/provider"
	"noto/internal/suggest"
)

// ProviderFunc sends a single chat turn and returns the assistant reply.
type ProviderFunc func(ctx context.Context, userMsg string) (string, error)

// ListModelsFunc fetches the available models from the configured provider.
type ListModelsFunc func(ctx context.Context) ([]provider.ModelInfo, error)

// ModelSelectedFunc is called when the user picks a model in the picker.
type ModelSelectedFunc func(modelID string) error

// Model is the root Bubble Tea model for Noto.
type Model struct {
	profileName  string
	activeModel  string // currently selected model (shown in header)
	input        textinput.Model
	viewport     viewport.Model
	messages     []chatMessage
	suggestions  []suggest.Suggestion
	width        int
	height       int
	err          error
	ready        bool

	// picker is non-nil when the /model overlay is active.
	picker *pickerState

	dispatcher    *chat.Dispatcher
	execCtx       *commands.ExecContext
	provider      ProviderFunc
	listModels    ListModelsFunc
	modelSelected ModelSelectedFunc
}

type chatMessage struct {
	role    string
	content string
}

// ---- async messages ---------------------------------------------------------

type providerReplyMsg struct{ content string; err error }
type modelsLoadedMsg  struct{ models []provider.ModelInfo; err error }

// ---- styles -----------------------------------------------------------------

var (
	headerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	suggStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	cmdOutStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	modelBadge     = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
)

// New creates a new TUI Model.
func New(
	profileName string,
	activeModel string,
	dispatcher *chat.Dispatcher,
	execCtx *commands.ExecContext,
	provider ProviderFunc,
	listModels ListModelsFunc,
	modelSelected ModelSelectedFunc,
) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message or /command…"
	ti.Focus()
	ti.CharLimit = 4096

	return Model{
		profileName:   profileName,
		activeModel:   activeModel,
		input:         ti,
		dispatcher:    dispatcher,
		execCtx:       execCtx,
		provider:      provider,
		listModels:    listModels,
		modelSelected: modelSelected,
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

	// ---- window size --------------------------------------------------------
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		headerH := 2
		inputH  := 2
		vpH := msg.Height - headerH - inputH - 2
		if vpH < 1 { vpH = 1 }
		if !m.ready {
			m.viewport = viewport.New(msg.Width, vpH)
			m.viewport.SetContent(m.renderHistory())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vpH
		}

	// ---- async: models list loaded ------------------------------------------
	case modelsLoadedMsg:
		if m.picker != nil {
			m.picker.loading = false
			if msg.err != nil {
				m.picker.err = msg.err
			} else {
				ids := make([]string, len(msg.models))
				for i, mi := range msg.models {
					ids[i] = mi.ID
				}
				m.picker.models = ids
				m.picker.cursor = 0
			}
		}

	// ---- async: provider reply ----------------------------------------------
	case providerReplyMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.messages = append(m.messages, chatMessage{role: "assistant", content: msg.content})
		}
		m.syncViewport()

	// ---- keyboard -----------------------------------------------------------
	case tea.KeyMsg:
		// Picker overlay handles its own keys.
		if m.picker != nil {
			return m.updatePicker(msg, cmds)
		}

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

			// /model opens the picker directly without going through the dispatcher.
			if val == "/model" || val == "/model " {
				return m.openModelPicker(cmds)
			}

			// All other input goes through the slash dispatcher.
			result := m.dispatcher.Dispatch(val, m.execCtx)

			if result.IsSlash {
				if result.Err != nil {
					m.err = result.Err
				} else if result.Executed && result.Output != "" {
					m.messages = append(m.messages, chatMessage{
						role:    "command",
						content: strings.TrimRight(result.Output, "\n"),
					})
				} else if !result.Executed && len(result.Suggestions) == 0 {
					// partial — do nothing, suggestions already cleared
				}
				m.syncViewport()
				return m, nil
			}

			// Plain chat.
			m.messages = append(m.messages, chatMessage{role: "user", content: val})
			m.syncViewport()

			if m.provider == nil {
				m.messages = append(m.messages, chatMessage{
					role:    "assistant",
					content: "No provider configured. Run: noto provider set --key <key>\nThen pick a model with /model",
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
	}

	// Forward remaining events to text input + refresh suggestions.
	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	cmds = append(cmds, inputCmd)

	val := m.input.Value()
	if strings.HasPrefix(val, "/") {
		// Use a trailing space to force "partial" mode in the dispatcher.
		result := m.dispatcher.Dispatch(val+" ", m.execCtx)
		m.suggestions = result.Suggestions
	} else {
		m.suggestions = nil
	}

	return m, tea.Batch(cmds...)
}

// updatePicker handles keypresses when the model picker overlay is open.
func (m Model) updatePicker(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.picker = nil
		return m, tea.Batch(cmds...)

	case tea.KeyEnter:
		chosen := m.picker.selected()
		m.picker = nil
		if chosen != "" && m.modelSelected != nil {
			if err := m.modelSelected(chosen); err != nil {
				m.err = err
			} else {
				m.activeModel = chosen
				m.messages = append(m.messages, chatMessage{
					role:    "command",
					content: "Model set to: " + chosen,
				})
				m.syncViewport()
			}
		}
		return m, tea.Batch(cmds...)

	case tea.KeyUp:
		if m.picker.cursor > 0 {
			m.picker.cursor--
		}

	case tea.KeyDown:
		list := m.picker.filtered()
		if m.picker.cursor < len(list)-1 {
			m.picker.cursor++
		}

	case tea.KeyBackspace:
		if len(m.picker.filter) > 0 {
			m.picker.filter = m.picker.filter[:len(m.picker.filter)-1]
			m.picker.cursor = 0
		}

	default:
		// Printable rune → add to filter.
		if msg.Type == tea.KeyRunes {
			m.picker.filter += msg.String()
			m.picker.cursor = 0
		}
	}

	m.picker.clampCursor()
	return m, tea.Batch(cmds...)
}

// openModelPicker initialises the picker overlay and fires the async models fetch.
func (m Model) openModelPicker(cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	m.picker = &pickerState{loading: true}
	if m.listModels != nil {
		cmds = append(cmds, func() tea.Msg {
			models, err := m.listModels(context.Background())
			return modelsLoadedMsg{models: models, err: err}
		})
	} else {
		m.picker.loading = false
		m.picker.err = errNoProvider
	}
	return m, tea.Batch(cmds...)
}

var errNoProvider = fmt.Errorf("no provider configured")

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "\n  Initialising…"
	}

	// Header.
	modelPart := ""
	if m.activeModel != "" {
		modelPart = "  " + modelBadge.Render("["+m.activeModel+"]")
	}
	header := headerStyle.Render("─── Noto · "+m.profileName+" ── /model to switch · Ctrl+C quit ───") + modelPart

	// Picker overlay replaces the suggestions area.
	var midSection string
	if m.picker != nil {
		pickerHeight := m.height / 2
		if pickerHeight < 6 { pickerHeight = 6 }
		midSection = m.picker.render(pickerHeight) + "\n"
	} else {
		// Slash suggestions.
		if len(m.suggestions) > 0 {
			var sb strings.Builder
			for _, s := range m.suggestions {
				sb.WriteString(suggStyle.Render("  /"+s.CommandPath+"  — "+s.Hint) + "\n")
			}
			midSection = sb.String()
		}
	}

	var errBlock string
	if m.err != nil {
		errBlock = errStyle.Render("  ✗ "+m.err.Error()) + "\n"
	}

	return header + "\n" +
		m.viewport.View() + "\n" +
		midSection +
		errBlock +
		m.input.View()
}

// syncViewport rebuilds viewport content and scrolls to the bottom.
func (m *Model) syncViewport() {
	if !m.ready { return }
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
			sb.WriteString(cmdOutStyle.Render("  "+msg.content) + "\n\n")
		}
	}
	return sb.String()
}

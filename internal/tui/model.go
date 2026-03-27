package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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

// NotesSaved returns a tea.Msg that tells the TUI to show the notes indicator.
// Call this from the NotesCallback passed to chat.NewSession.
func NotesSaved(count int) tea.Msg { return notesSavedMsg{count: count} }

// ListModelsFunc fetches the available models from the configured provider.
type ListModelsFunc func(ctx context.Context) ([]provider.ModelInfo, error)

// ModelSelectedFunc is called when the user picks a model in the picker.
type ModelSelectedFunc func(modelID string) error

// Model is the root Bubble Tea model for Noto.
type Model struct {
	profileName string
	activeModel string
	input       textinput.Model
	viewport    viewport.Model
	messages    []chatMessage
	width       int
	height      int
	err         error
	ready       bool

	// slash suggestion state
	suggestions []suggest.Suggestion
	suggCursor  int  // -1 = nothing selected; 0..n-1 = highlighted entry
	suggActive  bool // true when ↑/↓ navigation is in progress

	// model picker overlay (non-nil when /model is open)
	picker *pickerState

	// notesIndicator briefly shows a note-saved badge after extraction.
	notesIndicator string

	dispatcher    *chat.Dispatcher
	execCtx       *commands.ExecContext
	provider      ProviderFunc
	listModels    ListModelsFunc
	modelSelected ModelSelectedFunc
}

type chatMessage struct{ role, content string }

// ---- async tea.Msg types ----------------------------------------------------

type providerReplyMsg       struct{ content string; err error }
type modelsLoadedMsg        struct{ models []provider.ModelInfo; err error }
type notesSavedMsg          struct{ count int }
type clearNotesIndicatorMsg struct{}
type editorFinishedMsg      struct{ err error }
type profileChangedMsg      struct{ name string }

// ProfileChanged returns a tea.Msg that updates the profile name in the header.
func ProfileChanged(name string) tea.Msg { return profileChangedMsg{name: name} }

// ---- styles -----------------------------------------------------------------

var (
	headerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	userStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	assistantStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	cmdOutStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	modelBadge      = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	notesBadge      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	suggNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	suggSelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4")).Bold(true)
)

// New creates a new TUI Model.
func New(
	profileName string,
	activeModel string,
	dispatcher *chat.Dispatcher,
	execCtx *commands.ExecContext,
	providerFn ProviderFunc,
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
		suggCursor:    -1,
		dispatcher:    dispatcher,
		execCtx:       execCtx,
		provider:      providerFn,
		listModels:    listModels,
		modelSelected: modelSelected,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return textinput.Blink }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ---- window size --------------------------------------------------------
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		vpH := msg.Height - 4 // header + input rows
		if vpH < 1 {
			vpH = 1
		}
		if !m.ready {
			m.viewport = viewport.New(msg.Width, vpH)
			m.viewport.SetContent(m.renderHistory())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vpH
		}

	// ---- models loaded (for /model picker) ----------------------------------
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

	// ---- notes saved (background extraction) --------------------------------
	case notesSavedMsg:
		if msg.count > 0 {
			m.notesIndicator = fmt.Sprintf("📝 %d note(s) saved", msg.count)
			cmds = append(cmds, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
				return clearNotesIndicatorMsg{}
			}))
		}

	case clearNotesIndicatorMsg:
		m.notesIndicator = ""

	// ---- profile changed (select / rename) ---------------------------------
	case profileChangedMsg:
		m.profileName = msg.name

	// ---- editor finished ----------------------------------------------------
	case editorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.messages = append(m.messages, chatMessage{
				role:    "command",
				content: "System prompt updated.",
			})
			m.syncViewport()
		}

	// ---- provider reply -----------------------------------------------------
	case providerReplyMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.messages = append(m.messages, chatMessage{role: "assistant", content: msg.content})
		}
		m.syncViewport()

	// ---- keyboard -----------------------------------------------------------
	case tea.KeyMsg:
		// Model picker overlay takes priority.
		if m.picker != nil {
			return m.updatePicker(msg, cmds)
		}

		// Suggestion navigation takes priority over everything except Ctrl+C.
		if m.suggActive && len(m.suggestions) > 0 {
			return m.updateSuggNav(msg, cmds)
		}

		switch msg.Type {

		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEsc:
			// Dismiss suggestions if visible, otherwise quit.
			if len(m.suggestions) > 0 {
				m.clearSuggestions()
				m.input.SetValue("")
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyUp:
			// Start suggestion navigation if suggestions are showing.
			if len(m.suggestions) > 0 {
				m.suggActive = true
				m.suggCursor = len(m.suggestions) - 1
				m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
				m.input.CursorEnd()
				return m, nil
			}
			// Otherwise scroll viewport.
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd

		case tea.KeyDown:
			if len(m.suggestions) > 0 {
				m.suggActive = true
				m.suggCursor = 0
				m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
				m.input.CursorEnd()
				return m, nil
			}
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd

		case tea.KeyPgUp, tea.KeyPgDown:
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd

		case tea.KeyEnter:
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.clearSuggestions()
			m.err = nil
			return m.handleSubmit(val, cmds)
		}
	}

	// Forward to text input and recompute suggestions.
	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	cmds = append(cmds, inputCmd)
	m.refreshSuggestions()

	return m, tea.Batch(cmds...)
}

// updateSuggNav handles ↑/↓/Enter/Esc while suggestion navigation is active.
func (m Model) updateSuggNav(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.suggCursor > 0 {
			m.suggCursor--
		} else {
			m.suggCursor = len(m.suggestions) - 1
		}
		m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
		m.input.CursorEnd()

	case tea.KeyDown:
		if m.suggCursor < len(m.suggestions)-1 {
			m.suggCursor++
		} else {
			m.suggCursor = 0
		}
		m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
		m.input.CursorEnd()

	case tea.KeyEnter:
		val := strings.TrimSpace(m.input.Value())
		m.clearSuggestions()
		m.err = nil
		if val == "" {
			return m, tea.Batch(cmds...)
		}
		return m.handleSubmit(val, cmds)

	case tea.KeyEsc, tea.KeyCtrlC:
		m.clearSuggestions()
		m.input.SetValue("")
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	default:
		// Any other key exits navigation and lets the input handle it normally.
		m.suggActive = false
		m.suggCursor = -1
		var inputCmd tea.Cmd
		m.input, inputCmd = m.input.Update(msg)
		cmds = append(cmds, inputCmd)
		m.refreshSuggestions()
	}

	return m, tea.Batch(cmds...)
}

// handleSubmit processes a confirmed input value (Enter pressed).
func (m Model) handleSubmit(val string, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	// /model opens the model picker.
	if strings.TrimSpace(val) == "/model" {
		return m.openModelPicker(cmds)
	}

	// Route through slash dispatcher.
	result := m.dispatcher.Dispatch(val, m.execCtx)
	if result.IsSlash {
		if result.Err != nil {
			// Check if this is a request to open the editor.
			if openErr, ok := commands.AsErrOpenEditor(result.Err); ok {
				return m, m.openEditor(openErr.Path, cmds)
			}
			m.err = result.Err
		} else if result.Executed && result.Output != "" {
			m.messages = append(m.messages, chatMessage{
				role:    "command",
				content: strings.TrimRight(result.Output, "\n"),
			})
			m.syncViewport()
		}
		return m, tea.Batch(cmds...)
	}

	// Plain chat message.
	m.messages = append(m.messages, chatMessage{role: "user", content: val})
	m.syncViewport()

	if m.provider == nil {
		m.messages = append(m.messages, chatMessage{
			role:    "assistant",
			content: "No provider configured. Run: noto provider set --key <key>\nThen pick a model with /model",
		})
		m.syncViewport()
		return m, tea.Batch(cmds...)
	}

	userVal := val
	cmds = append(cmds, func() tea.Msg {
		reply, err := m.provider(context.Background(), userVal)
		return providerReplyMsg{content: reply, err: err}
	})
	return m, tea.Batch(cmds...)
}

// refreshSuggestions recomputes the suggestion list from the current input value.
func (m *Model) refreshSuggestions() {
	val := m.input.Value()
	if strings.HasPrefix(val, "/") && !m.suggActive {
		result := m.dispatcher.Dispatch(val+" ", m.execCtx)
		if len(result.Suggestions) != len(m.suggestions) {
			m.suggCursor = -1 // reset cursor when list changes
		}
		m.suggestions = result.Suggestions
	} else if !strings.HasPrefix(val, "/") {
		m.clearSuggestions()
	}
}

// clearSuggestions resets all suggestion state.
func (m *Model) clearSuggestions() {
	m.suggestions = nil
	m.suggCursor = -1
	m.suggActive = false
}

// openEditor suspends the TUI, opens the file in $EDITOR, then resumes.
func (m Model) openEditor(path string, cmds []tea.Cmd) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, path)
	cmds = append(cmds, tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	}))
	return tea.Batch(cmds...)
}

// openModelPicker initialises the picker overlay and fires the async fetch.
func (m Model) openModelPicker(cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	m.picker = &pickerState{loading: true}
	if m.listModels != nil {
		cmds = append(cmds, func() tea.Msg {
			models, err := m.listModels(context.Background())
			return modelsLoadedMsg{models: models, err: err}
		})
	} else {
		m.picker.loading = false
		m.picker.err = fmt.Errorf("no provider configured")
	}
	return m, tea.Batch(cmds...)
}

// updatePicker handles keypresses when the model picker overlay is open.
func (m Model) updatePicker(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.picker = nil
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

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
		if msg.Type == tea.KeyRunes {
			m.picker.filter += msg.String()
			m.picker.cursor = 0
		}
	}

	if m.picker != nil {
		m.picker.clampCursor()
	}
	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "\n  Initialising…"
	}

	modelPart := ""
	if m.activeModel != "" {
		modelPart = "  " + modelBadge.Render("["+m.activeModel+"]")
	}
	notesPart := ""
	if m.notesIndicator != "" {
		notesPart = "  " + notesBadge.Render(m.notesIndicator)
	}
	header := headerStyle.Render("─── Noto · "+m.profileName+" ── /model · Ctrl+C quit ───") + modelPart + notesPart

	// Middle section: model picker OR slash suggestions OR empty.
	var mid strings.Builder
	if m.picker != nil {
		ph := m.height / 2
		if ph < 6 {
			ph = 6
		}
		mid.WriteString(m.picker.render(ph) + "\n")
	} else if len(m.suggestions) > 0 {
		mid.WriteString(m.renderSuggestions())
	}

	var errBlock string
	if m.err != nil {
		errBlock = errStyle.Render("  ✗ "+m.err.Error()) + "\n"
	}

	return header + "\n" +
		m.viewport.View() + "\n" +
		mid.String() +
		errBlock +
		m.input.View()
}

// renderSuggestions draws the suggestion list with the active cursor highlighted.
func (m *Model) renderSuggestions() string {
	var sb strings.Builder
	for i, s := range m.suggestions {
		line := fmt.Sprintf("  /%s  — %s", s.CommandPath, s.Hint)
		if i == m.suggCursor {
			sb.WriteString(suggSelectStyle.Render(line) + "\n")
		} else {
			sb.WriteString(suggNormalStyle.Render(line) + "\n")
		}
	}
	return sb.String()
}

// syncViewport rebuilds viewport content and scrolls to bottom.
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
			sb.WriteString(cmdOutStyle.Render("  "+msg.content) + "\n\n")
		}
	}
	return sb.String()
}

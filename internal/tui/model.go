package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"noto/internal/suggest"
)

// ViewState represents the current screen state.
type ViewState int

const (
	ViewProfileSelect ViewState = iota
	ViewChat
)

// Model is the root Bubble Tea model for Noto.
type Model struct {
	state       ViewState
	profileName string
	input       textinput.Model
	messages    []chatMessage
	suggestions []suggest.Suggestion
	width       int
	height      int
	err         error

	// suggester is called with the slash prefix to produce suggestions.
	suggester func(prefix string) []suggest.Suggestion
}

type chatMessage struct {
	role    string
	content string
}

// Styles.
var (
	userStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	promptStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	suggStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)

// New creates a new TUI Model.
func New(profileName string, suggester func(string) []suggest.Suggestion) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message or /command..."
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 80

	return Model{
		state:       ViewChat,
		profileName: profileName,
		input:       ti,
		suggester:   suggester,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}
			m.messages = append(m.messages, chatMessage{role: "user", content: val})
			m.input.SetValue("")
			m.suggestions = nil
			// TODO: wire to actual chat pipeline / dispatcher
			m.messages = append(m.messages, chatMessage{
				role:    "assistant",
				content: "[provider not configured — reply stub]",
			})
			return m, nil
		}
	}

	// Update the text input and refresh slash suggestions.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	val := m.input.Value()
	if strings.HasPrefix(val, "/") && m.suggester != nil {
		prefix := val[1:]
		m.suggestions = m.suggester(prefix)
	} else {
		m.suggestions = nil
	}

	return m, cmd
}

// View implements tea.Model.
func (m Model) View() string {
	var sb strings.Builder

	// Header.
	sb.WriteString(promptStyle.Render(fmt.Sprintf("─── Noto · profile: %s ───\n\n", m.profileName)))

	// Chat history.
	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			sb.WriteString(userStyle.Render("You: ") + msg.content + "\n")
		case "assistant":
			sb.WriteString(assistantStyle.Render("Noto: ") + msg.content + "\n")
		}
	}

	sb.WriteString("\n")

	// Slash suggestions.
	if len(m.suggestions) > 0 {
		sb.WriteString(suggStyle.Render("Suggestions:\n"))
		for _, s := range m.suggestions {
			sb.WriteString(suggStyle.Render(fmt.Sprintf("  /%s  — %s\n", s.CommandPath, s.Hint)))
		}
		sb.WriteString("\n")
	}

	// Error.
	if m.err != nil {
		sb.WriteString(errStyle.Render("Error: "+m.err.Error()) + "\n\n")
	}

	// Input.
	sb.WriteString(m.input.View())

	return sb.String()
}

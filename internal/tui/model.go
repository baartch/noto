package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/provider"
	"noto/internal/store"
	"noto/internal/suggest"
)

// ---- callback types ---------------------------------------------------------

// ProviderFunc sends a single chat turn and returns the assistant reply.
type ProviderFunc func(ctx context.Context, userMsg string) (string, error)

// ListModelsFunc fetches the available models from the configured provider.
type ListModelsFunc func(ctx context.Context) ([]provider.ModelInfo, error)

// ModelSelectedFunc is called when the user picks a model in the picker.
type ModelSelectedFunc func(modelID string) error

// ListProfilesFunc returns all profiles for the profile picker.
type ListProfilesFunc func(ctx context.Context) ([]*store.Profile, error)

// ProfileSwitchCmd returns a command to switch profiles.
type ProfileSwitchCmd func(profileName string) tea.Cmd

// ListBackupsFunc returns backup timestamps for the active profile.
type ListBackupsFunc func(ctx context.Context) ([]string, error)

// BackupSelectedFunc is called when the user picks a backup timestamp.
type BackupSelectedFunc func(timestamp string) error

// ExtractorModelSelectedFunc is called when the user picks an extractor model.
type ExtractorModelSelectedFunc func(modelID string) error

// ---- public tea.Msg constructors --------------------------------------------

// NotesSaved returns a tea.Msg that shows the notes saved badge.
func NotesSaved(saved, updated int) tea.Msg { return notesSavedMsg{saved: saved, updated: updated} }

// NotesSaving returns a tea.Msg that shows the notes saving indicator.
func NotesSaving() tea.Msg { return notesSavingMsg{} }

// StatsUpdated returns a tea.Msg that updates the token/cost status in the footer.
func StatsUpdated(formatted string) tea.Msg { return statsUpdatedMsg{formatted: formatted} }

// ProfileSwitched updates the TUI state after switching profiles.
func ProfileSwitched(profileName, activeModel, extractorModel, cacheStatus, tokenStatus string, provider ProviderFunc, listModels ListModelsFunc, modelSelected ModelSelectedFunc, extractorModelSelected ExtractorModelSelectedFunc) profileSwitchedMsg {
	return profileSwitchedMsg{
		profileName:            profileName,
		activeModel:            activeModel,
		extractorModel:         extractorModel,
		cacheStatus:            cacheStatus,
		tokenStatus:            tokenStatus,
		provider:               provider,
		listModels:             listModels,
		modelSelected:          modelSelected,
		extractorModelSelected: extractorModelSelected,
	}
}

// ProfileSwitchFailed returns a message for failed profile switches.
func ProfileSwitchFailed(err error) tea.Msg {
	return profileSwitchFailedMsg{err: err}
}

// ---- async tea.Msg types (internal) ----------------------------------------

type providerReplyMsg struct {
	content string
	err     error
}
type modelsLoadedMsg struct {
	items []pickerItem
	err   error
}
type profilesLoadedMsg struct {
	items []pickerItem
	err   error
}
type backupsLoadedMsg struct {
	items []pickerItem
	err   error
}
type notesSavedMsg struct{ saved, updated int }
type notesSavingMsg struct{}
type clearNotesIndicatorMsg struct{}
type editorFinishedMsg struct{ err error }
type statsUpdatedMsg struct{ formatted string }
type profileSwitchedMsg struct {
	profileName            string
	activeModel            string
	extractorModel         string
	cacheStatus            string
	tokenStatus            string
	provider               ProviderFunc
	listModels             ListModelsFunc
	modelSelected          ModelSelectedFunc
	extractorModelSelected ExtractorModelSelectedFunc
}
type profileSwitchFailedMsg struct{ err error }
type spinnerTickMsg struct{}

// ---- picker kind ------------------------------------------------------------

type pickerKind int

const (
	pickerKindModel          pickerKind = iota
	pickerKindProfile        pickerKind = iota
	pickerKindBackup         pickerKind = iota
	pickerKindExtractorModel pickerKind = iota
)

// ---- TUI model --------------------------------------------------------------

// Model is the root Bubble Tea model for Noto.
type Model struct {
	profileName string
	activeModel string
	cacheStatus string
	tokenStatus string
	input       textarea.Model
	viewport    viewport.Model
	messages    []chatMessage
	width       int
	height      int
	err         error
	ready       bool

	// slash suggestion state
	suggestions []suggest.Suggestion
	suggCursor  int
	suggActive  bool

	// picker overlay
	picker     *pickerState
	pickerKind pickerKind

	// notes badge
	notesIndicator string

	// pending assistant state
	pending      bool
	spinnerIndex int

	dispatcher             *chat.Dispatcher
	execCtx                *commands.ExecContext
	provider               ProviderFunc
	listModels             ListModelsFunc
	modelSelected          ModelSelectedFunc
	listProfiles           ListProfilesFunc
	profileSwitch          ProfileSwitchCmd
	listBackups            ListBackupsFunc
	backupSelected         BackupSelectedFunc
	extractorModel         string
	extractorModelSelected ExtractorModelSelectedFunc
}

type chatMessage struct {
	role      string
	content   string
	timestamp time.Time
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ---- styles -----------------------------------------------------------------

var (
	headerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(false)
	errStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	suggNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	suggSelectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63")).Bold(true)
	dividerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
)

// New creates a new TUI Model.
func New(
	profileName string,
	activeModel string,
	extractorModel string,
	cacheStatus string,
	tokenStatus string,
	dispatcher *chat.Dispatcher,
	execCtx *commands.ExecContext,
	providerFn ProviderFunc,
	listModels ListModelsFunc,
	modelSelected ModelSelectedFunc,
	listProfiles ListProfilesFunc,
	profileSwitch ProfileSwitchCmd,
	listBackups ListBackupsFunc,
	backupSelected BackupSelectedFunc,
	extractorModelSelected ExtractorModelSelectedFunc,
) Model {
	ti := textarea.New()
	ti.Placeholder = "Type a message or /command…"
	ti.Focus()
	ti.CharLimit = 4096
	ti.ShowLineNumbers = false
	ti.SetHeight(3)
	ti.Prompt = "  "
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ti.FocusedStyle.Prompt = promptStyle
	ti.BlurredStyle.Prompt = promptStyle
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	// Enter sends; Alt+Enter inserts newline.
	ti.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("alt+enter"),
		key.WithHelp("alt+enter", "insert newline"),
	)

	return Model{
		profileName:            profileName,
		activeModel:            activeModel,
		cacheStatus:            cacheStatus,
		tokenStatus:            tokenStatus,
		input:                  ti,
		suggCursor:             -1,
		dispatcher:             dispatcher,
		execCtx:                execCtx,
		provider:               providerFn,
		listModels:             listModels,
		modelSelected:          modelSelected,
		listProfiles:           listProfiles,
		profileSwitch:          profileSwitch,
		listBackups:            listBackups,
		backupSelected:         backupSelected,
		extractorModel:         extractorModel,
		extractorModelSelected: extractorModelSelected,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return textarea.Blink }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(msg.Width - 4)
		// header(1) + divider(1) + inputDivider(1) + inputLine(1) + padding(1) = 5
		vpH := msg.Height - 5 // inputDivider+input+hint+footer
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

	// ---- picker items loaded ------------------------------------------------
	case modelsLoadedMsg:
		if m.picker != nil && (m.pickerKind == pickerKindModel || m.pickerKind == pickerKindExtractorModel) {
			m.picker.loading = false
			if msg.err != nil {
				m.picker.err = msg.err
			} else {
				m.picker.items = msg.items
				m.picker.cursor = 0
			}
		}

	case profilesLoadedMsg:
		if m.picker != nil && m.pickerKind == pickerKindProfile {
			m.picker.loading = false
			if msg.err != nil {
				m.picker.err = msg.err
			} else {
				m.picker.items = msg.items
				m.picker.cursor = 0
			}
		}

	case backupsLoadedMsg:
		if m.picker != nil && m.pickerKind == pickerKindBackup {
			m.picker.loading = false
			if msg.err != nil {
				m.picker.err = msg.err
			} else {
				m.picker.items = msg.items
				m.picker.cursor = 0
			}
		}

	// ---- notes badge --------------------------------------------------------
	case notesSavedMsg:
		saved := msg.saved
		updated := msg.updated
		switch {
		case saved > 0 && updated > 0:
			m.notesIndicator = fmt.Sprintf("📝 %d saved, %d updated", saved, updated)
		case saved > 0:
			m.notesIndicator = fmt.Sprintf("📝 %d note(s) saved", saved)
		case updated > 0:
			m.notesIndicator = fmt.Sprintf("📝 %d note(s) updated", updated)
		default:
			m.notesIndicator = ""
		}
		if saved+updated > 0 {
			cmds = append(cmds, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
				return clearNotesIndicatorMsg{}
			}))
		}

	case notesSavingMsg:
		m.notesIndicator = "📝 validating…"

	case clearNotesIndicatorMsg:
		m.notesIndicator = ""

	// ---- editor finished ----------------------------------------------------
	case editorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.messages = append(m.messages, chatMessage{role: "command", content: "System prompt updated.", timestamp: time.Now()})
			m.syncViewport()
		}

	// ---- stats update -------------------------------------------------------
	case statsUpdatedMsg:
		m.tokenStatus = msg.formatted

	case profileSwitchedMsg:
		m.profileName = msg.profileName
		m.activeModel = msg.activeModel
		m.extractorModel = msg.extractorModel
		m.cacheStatus = msg.cacheStatus
		m.tokenStatus = msg.tokenStatus
		m.provider = msg.provider
		m.listModels = msg.listModels
		m.modelSelected = msg.modelSelected
		m.extractorModelSelected = msg.extractorModelSelected
		m.messages = []chatMessage{{role: "command", content: "Switched to profile: " + msg.profileName, timestamp: time.Now()}}
		m.err = nil
		m.clearSuggestions()
		m.input.SetValue("")
		m.syncViewport()

	case profileSwitchFailedMsg:
		m.err = msg.err
		m.syncViewport()

	case spinnerTickMsg:
		if m.pending {
			m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)
			m.updatePendingSpinner()
			m.syncViewport()
			cmds = append(cmds, tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} }))
		}

	// ---- provider reply -----------------------------------------------------
	case providerReplyMsg:
		if msg.err != nil {
			m.err = msg.err
			m.clearPending()
			m.pending = false
		} else {
			m.resolvePending(msg.content)
			m.pending = false
		}
		m.syncViewport()

	// ---- keyboard -----------------------------------------------------------
	case tea.KeyMsg:
		if m.picker != nil {
			return m.updatePicker(msg, cmds)
		}
		if m.suggActive && len(m.suggestions) > 0 {
			return m.updateSuggNav(msg, cmds)
		}

		//exhaustive:ignore
		switch msg.Type {
		case tea.KeyCtrlC:
			m.input.SetValue("")
			m.clearSuggestions()
			return m, nil

		case tea.KeyCtrlD:
			return m, tea.Quit

		case tea.KeyEsc:
			if len(m.suggestions) > 0 {
				m.clearSuggestions()
				m.input.SetValue("")
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyUp:
			if len(m.suggestions) > 0 {
				m.suggActive = true
				m.suggCursor = len(m.suggestions) - 1
				m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
				m.input.CursorEnd()
				return m, nil
			}
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
			// Send on Enter (newline handled by textarea only via Alt+Enter)
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

	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	cmds = append(cmds, inputCmd)
	m.refreshSuggestions()
	m.updatePendingSpinner()

	return m, tea.Batch(cmds...)
}

// updateSuggNav handles keyboard while suggestion navigation is active.
func (m Model) updateSuggNav(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	//exhaustive:ignore
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

	case tea.KeyEsc, tea.KeyCtrlC, tea.KeyCtrlD:
		m.clearSuggestions()
		m.input.SetValue("")
		if msg.Type == tea.KeyCtrlD {
			return m, tea.Quit
		}
		if msg.Type == tea.KeyCtrlC {
			return m, nil
		}

	default:
		m.suggActive = false
		m.suggCursor = -1
		var inputCmd tea.Cmd
		m.input, inputCmd = m.input.Update(msg)
		cmds = append(cmds, inputCmd)
		m.refreshSuggestions()
	}

	return m, tea.Batch(cmds...)
}

// handleSubmit processes a confirmed input value.
func (m Model) handleSubmit(val string, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(val) == "/model" {
		return m.openPicker(pickerKindModel, cmds)
	}
	if strings.TrimSpace(val) == "/provider extractor-model" {
		return m.openPicker(pickerKindExtractorModel, cmds)
	}

	result := m.dispatcher.Dispatch(val, m.execCtx)
	if result.IsSlash {
		if result.Err != nil {
			if openErr, ok := commands.AsErrOpenEditor(result.Err); ok {
				return m, m.openEditor(openErr.Path, cmds)
			}
			if commands.AsErrOpenProfilePicker(result.Err) {
				return m.openPicker(pickerKindProfile, cmds)
			}
			if commands.AsErrOpenBackupPicker(result.Err) {
				return m.openPicker(pickerKindBackup, cmds)
			}
			if commands.AsErrOpenExtractorModelPicker(result.Err) {
				return m.openPicker(pickerKindExtractorModel, cmds)
			}
			m.err = result.Err
		} else if result.Executed && result.Output != "" {
			m.messages = append(m.messages, chatMessage{
				role:      "command",
				content:   strings.TrimRight(result.Output, "\n"),
				timestamp: time.Now(),
			})
			m.syncViewport()
		}
		return m, tea.Batch(cmds...)
	}

	// Plain chat message.
	m.messages = append(m.messages, chatMessage{role: "user", content: val, timestamp: time.Now()})
	m.messages = append(m.messages, chatMessage{role: "pending", content: spinnerFrames[m.spinnerIndex], timestamp: time.Now()})
	m.pending = true
	m.syncViewport()
	cmds = append(cmds, tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerTickMsg{} }))

	if m.provider == nil {
		m.pending = false
		m.clearPending()
		m.messages = append(m.messages, chatMessage{
			role:      "assistant",
			content:   "No provider configured. Run: `noto provider set --key <key>`\nThen pick a model with `/model`",
			timestamp: time.Now(),
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

// openPicker initialises the picker overlay and fires the async data fetch.
func (m Model) openPicker(kind pickerKind, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	m.pickerKind = kind
	switch kind {
	case pickerKindModel:
		m.picker = &pickerState{title: "Select model", loading: true, width: m.width - 4}
		if m.listModels != nil {
			cmds = append(cmds, func() tea.Msg {
				models, err := m.listModels(context.Background())
				if err != nil {
					return modelsLoadedMsg{err: err}
				}
				items := make([]pickerItem, len(models))
				for i, mi := range models {
					items[i] = pickerItem{Value: mi.ID}
				}
				return modelsLoadedMsg{items: items}
			})
		} else {
			m.picker.loading = false
			m.picker.err = fmt.Errorf("no provider configured")
		}

	case pickerKindProfile:
		m.picker = &pickerState{title: "Select profile", loading: true, width: m.width - 4}
		if m.listProfiles != nil {
			current := m.profileName
			cmds = append(cmds, func() tea.Msg {
				profiles, err := m.listProfiles(context.Background())
				if err != nil {
					return profilesLoadedMsg{err: err}
				}
				items := make([]pickerItem, len(profiles))
				for i, p := range profiles {
					items[i] = pickerItem{
						Value:  p.Name,
						Active: p.Name == current,
					}
				}
				return profilesLoadedMsg{items: items}
			})
		} else {
			m.picker.loading = false
			m.picker.err = fmt.Errorf("no profile service available")
		}

	case pickerKindBackup:
		m.picker = &pickerState{title: "Restore backup", loading: true, width: m.width - 4}
		if m.listBackups != nil {
			cmds = append(cmds, func() tea.Msg {
				backups, err := m.listBackups(context.Background())
				if err != nil {
					return backupsLoadedMsg{err: err}
				}
				items := make([]pickerItem, len(backups))
				for i, ts := range backups {
					items[i] = pickerItem{Value: ts, Label: formatBackupTimestamp(ts)}
				}
				return backupsLoadedMsg{items: items}
			})
		} else {
			m.picker.loading = false
			m.picker.err = fmt.Errorf("no backup service available")
		}
	case pickerKindExtractorModel:
		m.picker = &pickerState{title: "Select extractor model", loading: true, width: m.width - 4}
		if m.listModels != nil {
			current := m.extractorModel
			cmds = append(cmds, func() tea.Msg {
				models, err := m.listModels(context.Background())
				if err != nil {
					return modelsLoadedMsg{err: err}
				}
				items := make([]pickerItem, len(models))
				for i, mi := range models {
					items[i] = pickerItem{Value: mi.ID, Active: mi.ID == current}
				}
				return modelsLoadedMsg{items: items}
			})
		} else {
			m.picker.loading = false
			m.picker.err = fmt.Errorf("no provider configured")
		}
	}

	return m, tea.Batch(cmds...)
}

// updatePicker handles keypresses while the picker overlay is open.
func (m Model) updatePicker(msg tea.KeyMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	//exhaustive:ignore
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC, tea.KeyCtrlD:
		if msg.Type == tea.KeyCtrlD {
			return m, tea.Quit
		}
		m.picker = nil
		if msg.Type == tea.KeyCtrlC {
			m.input.SetValue("")
			m.clearSuggestions()
			return m, nil
		}

	case tea.KeyEnter:
		chosen := m.picker.selectedValue()
		kind := m.pickerKind
		m.picker = nil
		m.input.SetValue("")
		m.clearSuggestions()
		if chosen == "" {
			return m, tea.Batch(cmds...)
		}
		switch kind {
		case pickerKindModel:
			if m.modelSelected != nil {
				if err := m.modelSelected(chosen); err != nil {
					m.err = err
				} else {
					m.activeModel = chosen
					m.messages = append(m.messages, chatMessage{role: "command", content: "Model set to: " + chosen, timestamp: time.Now()})
					m.syncViewport()
				}
			}
		case pickerKindProfile:
			if m.profileSwitch != nil {
				cmds = append(cmds, m.profileSwitch(chosen))
			}
		case pickerKindBackup:
			if m.backupSelected != nil {
				if err := m.backupSelected(chosen); err != nil {
					m.err = err
				} else {
					m.messages = append(m.messages, chatMessage{role: "command", content: "Restored backup: " + chosen, timestamp: time.Now()})
					m.syncViewport()
				}
			}
		case pickerKindExtractorModel:
			if m.extractorModelSelected != nil {
				if err := m.extractorModelSelected(chosen); err != nil {
					m.err = err
				} else {
					m.extractorModel = chosen
					m.messages = append(m.messages, chatMessage{role: "command", content: "Extractor model set to: " + chosen, timestamp: time.Now()})
					m.syncViewport()
				}
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

// openEditor suspends the TUI and opens a file in $EDITOR via tea.ExecProcess.
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

// refreshSuggestions recomputes the suggestion list from current input.
func (m *Model) refreshSuggestions() {
	val := m.input.Value()
	if strings.HasPrefix(val, "/") && !m.suggActive {
		result := m.dispatcher.Dispatch(val+" ", m.execCtx)
		if len(result.Suggestions) != len(m.suggestions) {
			m.suggCursor = -1
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

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing…"
	}

	// ---- middle: picker or suggestions ----
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

	// ---- input bar ----
	inputDivider := dividerStyle.Render(strings.Repeat("─", m.width))
	inputView := strings.TrimRight(m.input.View(), "\n")
	inputLine := lipgloss.NewStyle().PaddingLeft(1).Render(inputView)

	footer := m.renderFooter()

	return m.viewport.View() + "\n" +
		mid.String() +
		errBlock +
		inputDivider + "\n" +
		inputLine + "\n" +
		footer
}

// renderFooter draws the bottom status line.
func (m *Model) renderFooter() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("71"))
	blue := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	purple := lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	white := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// Left: token stats + cache status + notes badge.
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

	left := strings.Join(leftParts, dim.Render("  "))

	// Right: profile + model.
	right := white.Render(m.profileName)
	if m.activeModel != "" {
		right = right + dim.Render("  ") + purple.Render("["+m.activeModel+"]")
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

func formatBackupTimestamp(ts string) string {
	parsed, err := time.Parse("20060102T150405Z", ts)
	if err != nil {
		return ts
	}
	return parsed.UTC().Format("2006-01-02 15:04:05 UTC")
}

// renderSuggestions draws the suggestion list with cursor highlighted.
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

func (m *Model) resolvePending(content string) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].role == "pending" {
			m.messages[i].role = "assistant"
			m.messages[i].content = content
			m.messages[i].timestamp = time.Now()
			return
		}
	}
	m.messages = append(m.messages, chatMessage{role: "assistant", content: content, timestamp: time.Now()})
}

func (m *Model) clearPending() {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].role == "pending" {
			m.messages = append(m.messages[:i], m.messages[i+1:]...)
			return
		}
	}
}

func (m *Model) updatePendingSpinner() {
	if !m.pending {
		return
	}
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].role == "pending" {
			m.messages[i].content = spinnerFrames[m.spinnerIndex]
			return
		}
	}
}

// syncViewport rebuilds viewport content and scrolls to bottom.
func (m *Model) syncViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
}

// renderHistory renders all chat messages using styled bubbles.
func (m *Model) renderHistory() string {
	if len(m.messages) == 0 {
		return "\n" + headerStyle.Render("  No messages yet — start typing below.") + "\n"
	}

	termWidth := m.width
	if termWidth < 40 {
		termWidth = 80 // safe default before WindowSizeMsg
	}

	var sb strings.Builder
	for _, msg := range m.messages {
		ts := msg.timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		switch msg.role {
		case "user":
			sb.WriteString(renderUserBubble(msg.content, "You", ts, termWidth))
		case "assistant":
			sb.WriteString(renderAssistantBubble(msg.content, m.activeModel, ts, termWidth))
		case "pending":
			sb.WriteString(renderAssistantBubble(msg.content, m.activeModel, ts, termWidth))
		case "command":
			sb.WriteString(renderCommandLine(msg.content))
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/profile"
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
func ProfileSwitched(profileName, activeModel, extractorModel, cacheStatus, tokenStatus string, extractorFallback bool, provider ProviderFunc, listModels ListModelsFunc, modelSelected ModelSelectedFunc, extractorModelSelected ExtractorModelSelectedFunc, settings *SettingsMenu, history []string) profileSwitchedMsg {
	return profileSwitchedMsg{
		profileName:            profileName,
		activeModel:            activeModel,
		extractorModel:         extractorModel,
		cacheStatus:            cacheStatus,
		tokenStatus:            tokenStatus,
		extractorFallback:      extractorFallback,
		provider:               provider,
		listModels:             listModels,
		modelSelected:          modelSelected,
		extractorModelSelected: extractorModelSelected,
		settings:               settings,
		history:                history,
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
type editorFinishedMsg struct {
	err    error
	onSave func() error
}
type statsUpdatedMsg struct{ formatted string }
type profileSwitchedMsg struct {
	profileName            string
	activeModel            string
	extractorModel         string
	cacheStatus            string
	tokenStatus            string
	extractorFallback      bool
	provider               ProviderFunc
	listModels             ListModelsFunc
	modelSelected          ModelSelectedFunc
	extractorModelSelected ExtractorModelSelectedFunc
	settings               *SettingsMenu
	history                []string
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

const (
	settingsHeaderText = "Settings"
	settingsHelpText   = "  Settings (↑↓ navigate · Enter edit/select · Esc close)"
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
	keys        keyMap
	help        help.Model
	helpKeys    helpKeyMap

	// slash suggestion state
	suggestions []suggest.Suggestion
	suggCursor  int
	suggActive  bool

	// input history
	history      []string
	historyIndex int
	historyDraft string

	// picker overlay
	picker     *pickerState
	pickerKind pickerKind

	// settings dialog
	settingsOpen bool
	settingsMenu *SettingsMenu

	// settings editor
	settingsList      list.Model
	settingsEditing   bool
	settingsEditor    textarea.Model
	settingsEditEntry *SettingsEntry
	settingsErr       string

	// profile settings
	memoryTokenBudget int
	systemPrompt      string

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
	extractorFallback      bool
}

type chatMessage struct {
	role      string
	content   string
	timestamp time.Time
}

type keyMap struct {
	quit         key.Binding
	openModel    key.Binding
	clearInput   key.Binding
	toggleHelp   key.Binding
	openSettings key.Binding
}

type helpKeyMap struct {
	primary   []key.Binding
	secondary []key.Binding
}

func (h helpKeyMap) ShortHelp() []key.Binding {
	return h.primary
}

func (h helpKeyMap) FullHelp() [][]key.Binding {
	if len(h.secondary) == 0 {
		return [][]key.Binding{h.primary}
	}
	return [][]key.Binding{h.primary, h.secondary}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ---- styles -----------------------------------------------------------------

// InputPlaceholder exposes the current textarea placeholder for tests.
func (m Model) InputPlaceholder() string {
	return m.input.Placeholder
}

// ViewportHeight exposes the viewport height for tests.
func (m Model) ViewportHeight() int {
	return m.viewport.Height()
}

// New creates a new TUI Model.
func New(
	profileName string,
	activeModel string,
	extractorModel string,
	cacheStatus string,
	tokenStatus string,
	extractorFallback bool,
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
	inputHistory []string,
) Model {
	ti := textarea.New()
	ti.Placeholder = "Type a message or /command…"
	ti.Focus()
	ti.CharLimit = 4096
	ti.ShowLineNumbers = false
	ti.SetHeight(3)
	ti.Prompt = "  "
	styles := ti.Styles()
	styles.Focused.Prompt = promptStyle
	styles.Blurred.Prompt = promptStyle
	styles.Cursor = cursorStyleDef
	ti.SetStyles(styles)

	settingsList := newSettingsList(30)
	settingsEditor := newSettingsEditor()
	// Enter sends; Alt+Enter inserts newline.
	ti.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("alt+enter"),
		key.WithHelp("alt+enter", "insert newline"),
	)
	keys := keyMap{
		quit:         key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "quit")),
		openModel:    key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "model picker")),
		clearInput:   key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "clear")),
		toggleHelp:   key.NewBinding(key.WithKeys("ctrl+h"), key.WithHelp("ctrl+h", "help")),
		openSettings: key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "settings")),
	}
	helpModel := help.New()
	helpModel.Styles.ShortKey = helpShortStyle
	helpModel.Styles.ShortDesc = helpShortStyle
	helpModel.Styles.FullKey = helpFullStyle
	helpModel.Styles.FullDesc = helpFullStyle
	helpKeys := helpKeyMap{
		primary: []key.Binding{keys.toggleHelp},
		secondary: []key.Binding{
			keys.openSettings,
			keys.openModel,
			keys.quit,
			keys.clearInput,
		},
	}

	if inputHistory == nil {
		inputHistory = []string{}
	}

	return Model{
		profileName:            profileName,
		activeModel:            activeModel,
		cacheStatus:            cacheStatus,
		tokenStatus:            tokenStatus,
		input:                  ti,
		suggCursor:             -1,
		history:                inputHistory,
		historyIndex:           len(inputHistory),
		dispatcher:             dispatcher,
		execCtx:                execCtx,
		provider:               providerFn,
		keys:                   keys,
		help:                   helpModel,
		helpKeys:               helpKeys,
		listModels:             listModels,
		modelSelected:          modelSelected,
		listProfiles:           listProfiles,
		profileSwitch:          profileSwitch,
		listBackups:            listBackups,
		backupSelected:         backupSelected,
		extractorModel:         extractorModel,
		extractorModelSelected: extractorModelSelected,
		extractorFallback:      extractorFallback,
		settingsMenu:           DefaultSettingsMenu(),
		settingsList:           settingsList,
		settingsEditor:         settingsEditor,
		settingsErr:            "",
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
		vpH = max(vpH, 1)
		if !m.ready {
			m.viewport = viewport.New(viewport.WithWidth(msg.Width), viewport.WithHeight(vpH))
			m.viewport.SetContent(m.renderHistory())
			m.ready = true
		} else {
			m.viewport.SetWidth(msg.Width)
			m.viewport.SetHeight(vpH)
		}

	// ---- picker items loaded ------------------------------------------------
	case modelsLoadedMsg:
		if m.picker != nil && (m.pickerKind == pickerKindModel || m.pickerKind == pickerKindExtractorModel) {
			m.picker.loading = false
			if msg.err != nil {
				m.picker.err = msg.err
			} else {
				m.picker.setItems(msg.items)
			}
		}

	case profilesLoadedMsg:
		if m.picker != nil && m.pickerKind == pickerKindProfile {
			m.picker.loading = false
			if msg.err != nil {
				m.picker.err = msg.err
			} else {
				m.picker.setItems(msg.items)
			}
		}

	case backupsLoadedMsg:
		if m.picker != nil && m.pickerKind == pickerKindBackup {
			m.picker.loading = false
			if msg.err != nil {
				m.picker.err = msg.err
			} else {
				m.picker.setItems(msg.items)
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
		} else if msg.onSave != nil {
			if err := msg.onSave(); err != nil {
				m.err = err
			} else {
				m.messages = append(m.messages, chatMessage{role: "command", content: "System prompt updated.", timestamp: time.Now()})
				m.syncViewport()
			}
		}

	// ---- stats update -------------------------------------------------------
	case statsUpdatedMsg:
		m.tokenStatus = msg.formatted

	case profileSwitchedMsg:
		m.profileName = msg.profileName
		m.activeModel = msg.activeModel
		m.extractorModel = msg.extractorModel
		m.extractorFallback = msg.extractorFallback
		m.cacheStatus = msg.cacheStatus
		m.tokenStatus = msg.tokenStatus
		m.provider = msg.provider
		m.listModels = msg.listModels
		m.modelSelected = msg.modelSelected
		m.extractorModelSelected = msg.extractorModelSelected
		m.settingsMenu = msg.settings
		m.memoryTokenBudget = 0
		m.systemPrompt = ""
		m.settingsEditEntry = nil
		m.settingsEditing = false
		m.settingsErr = ""
		m.history = msg.history
		m.historyIndex = len(msg.history)
		m.historyDraft = ""
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

	// ---- picker: forward all non-key messages (e.g. FilterMatchesMsg) --------
	default:
		if m.picker != nil {
			updated, cmd := m.picker.list.Update(msg)
			m.picker.list = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	// ---- keyboard -----------------------------------------------------------
	case tea.KeyPressMsg:
		if m.settingsOpen && m.settingsEditing {
			switch msg.Key().Code {
			case tea.KeyEsc:
				m.settingsEditing = false
				m.settingsEditEntry = nil
				m.settingsErr = ""
				return m, nil
			case tea.KeyEnter:
				return m.handleSettingsSave()
			}
			var cmd tea.Cmd
			m.settingsEditor, cmd = m.settingsEditor.Update(msg)
			return m, cmd
		}
		if m.picker != nil {
			return m.updatePicker(msg, cmds)
		}
		if m.suggActive && len(m.suggestions) > 0 {
			return m.updateSuggNav(msg, cmds)
		}

		//exhaustive:ignore
		switch {
		case key.Matches(msg, m.keys.clearInput):
			m.input.SetValue("")
			m.clearSuggestions()
			return m, nil

		case key.Matches(msg, m.keys.quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.openModel):
			return m.openPicker(pickerKindModel, cmds)

		case key.Matches(msg, m.keys.toggleHelp):
			m.help.ShowAll = !m.help.ShowAll
			return m, nil

		case key.Matches(msg, m.keys.openSettings) || msg.String() == "ctrl+j" || msg.Key().Keystroke() == "ctrl+j":
			m.clearSuggestions()
			m.input.SetValue("")
			m.settingsOpen = !m.settingsOpen
			if m.settingsOpen {
				m.settingsEditing = false
				m.settingsErr = ""
				if m.settingsMenu == nil {
					m.settingsMenu = DefaultSettingsMenu()
				}
				m.refreshSettingsValues()
			}
			return m, nil

		case msg.Key().Code == tea.KeyEsc:
			if m.settingsOpen {
				m.settingsOpen = false
				m.settingsEditing = false
				m.settingsErr = ""
				return m, nil
			}
			if len(m.suggestions) > 0 {
				m.clearSuggestions()
				m.input.SetValue("")
				return m, nil
			}
			return m, tea.Quit

		case msg.Key().Code == tea.KeyUp:
			if m.settingsOpen && !m.settingsEditing {
				var cmd tea.Cmd
				m.settingsList, cmd = m.settingsList.Update(msg)
				return m, cmd
			}
			if len(m.suggestions) > 0 {
				m.suggActive = true
				m.suggCursor = len(m.suggestions) - 1
				m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
				m.input.CursorEnd()
				return m, nil
			}
			if m.navigateHistory(-1) {
				return m, nil
			}
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd

		case msg.Key().Code == tea.KeyDown:
			if m.settingsOpen && !m.settingsEditing {
				var cmd tea.Cmd
				m.settingsList, cmd = m.settingsList.Update(msg)
				return m, cmd
			}
			if len(m.suggestions) > 0 {
				m.suggActive = true
				m.suggCursor = 0
				m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
				m.input.CursorEnd()
				return m, nil
			}
			if m.navigateHistory(1) {
				return m, nil
			}
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd

		case msg.Key().Code == tea.KeyPgUp, msg.Key().Code == tea.KeyPgDown:
			if m.settingsOpen && !m.settingsEditing {
				var cmd tea.Cmd
				m.settingsList, cmd = m.settingsList.Update(msg)
				return m, cmd
			}
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd

		case msg.Key().Code == tea.KeyTab:
			if len(m.suggestions) > 0 {
				if m.suggCursor < 0 {
					m.suggCursor = 0
				}
				m.suggActive = true
				m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
				m.input.CursorEnd()
				return m, nil
			}

		case msg.Key().Code == tea.KeyEnter:
			if m.settingsOpen {
				return m.handleSettingsEnter()
			}
			// Send on Enter (newline handled by textarea only via Alt+Enter)
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.clearSuggestions()
			m.err = nil
			m.recordHistory(val)
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
func (m Model) updateSuggNav(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	//exhaustive:ignore
	switch {
	case msg.Key().Code == tea.KeyUp:
		if m.suggCursor > 0 {
			m.suggCursor--
		}
		m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
		m.input.CursorEnd()

	case msg.Key().Code == tea.KeyDown:
		if m.suggCursor < len(m.suggestions)-1 {
			m.suggCursor++
		}
		m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
		m.input.CursorEnd()

	case msg.Key().Code == tea.KeyTab:
		if len(m.suggestions) > 0 {
			if m.suggCursor < 0 {
				m.suggCursor = 0
			}
			m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
			m.input.CursorEnd()
			return m, tea.Batch(cmds...)
		}

	case msg.Key().Code == tea.KeyEnter:
		val := strings.TrimSpace(m.input.Value())
		m.clearSuggestions()
		m.err = nil
		if val == "" {
			return m, tea.Batch(cmds...)
		}
		return m.handleSubmit(val, cmds)

	case msg.Key().Code == tea.KeyEsc:
		m.clearSuggestions()
		m.input.SetValue("")
	case key.Matches(msg, m.keys.quit):
		m.clearSuggestions()
		m.input.SetValue("")
		return m, tea.Quit
	case key.Matches(msg, m.keys.clearInput):
		m.clearSuggestions()
		m.input.SetValue("")
		return m, nil
	case key.Matches(msg, m.keys.openModel):
		m.clearSuggestions()
		m.input.SetValue("")
		return m.openPicker(pickerKindModel, cmds)
	case key.Matches(msg, m.keys.toggleHelp):
		m.clearSuggestions()
		m.input.SetValue("")
		m.help.ShowAll = !m.help.ShowAll
		return m, nil

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
	if strings.TrimSpace(val) == "/model extractor" {
		return m.openPicker(pickerKindExtractorModel, cmds)
	}

	result := m.dispatcher.Dispatch(val, m.execCtx)
	if result.IsSlash {
		if result.Err != nil {
			if openErr, ok := commands.AsErrOpenEditor(result.Err); ok {
				return m, m.openEditor(openErr.Path, openErr.OnSave, cmds)
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

// openPicker initializes the picker overlay and fires the async data fetch.
func (m Model) openPicker(kind pickerKind, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	m.pickerKind = kind
	m.input.SetValue("")
	m.input.Blur()
	switch kind {
	case pickerKindModel:
		m.picker = newPickerState("Select model", m.width-4)
		m.picker.loading = true
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
			m.picker.err = errors.New("no provider configured")
		}

	case pickerKindProfile:
		m.picker = newPickerState("Select profile", m.width-4)
		m.picker.loading = true
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
			m.picker.err = errors.New("no profile service available")
		}

	case pickerKindBackup:
		m.picker = newPickerState("Restore backup", m.width-4)
		m.picker.loading = true
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
			m.picker.err = errors.New("no backup service available")
		}
	case pickerKindExtractorModel:
		m.picker = newPickerState("Select extractor model", m.width-4)
		m.picker.loading = true
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
			m.picker.err = errors.New("no provider configured")
		}
	}

	return m, tea.Batch(cmds...)
}

// updatePicker handles keypresses while the picker overlay is open.
func (m Model) updatePicker(msg tea.KeyPressMsg, cmds []tea.Cmd) (tea.Model, tea.Cmd) {
	//exhaustive:ignore
	switch {
	case msg.Key().Code == tea.KeyEsc:
		m.picker = nil
		cmds = append(cmds, m.input.Focus())
	case key.Matches(msg, m.keys.quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.clearInput):
		m.picker = nil
		m.input.SetValue("")
		m.clearSuggestions()
		return m, m.input.Focus()
	case key.Matches(msg, m.keys.openModel):
		m.picker = nil
		m.input.SetValue("")
		m.clearSuggestions()
		return m.openPicker(pickerKindModel, cmds)
	case key.Matches(msg, m.keys.toggleHelp):
		m.help.ShowAll = !m.help.ShowAll
		return m, nil

	case msg.Key().Code == tea.KeyEnter:
		chosen := m.picker.selectedValue()
		kind := m.pickerKind
		m.picker = nil
		m.input.SetValue("")
		m.clearSuggestions()
		cmds = append(cmds, m.input.Focus())
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

	default:
		if m.picker != nil {
			ph := max(m.height/2, 6)
			maxRows := max(ph-2, 5)
			m.picker.list.SetSize(max(m.width-2, 10), maxRows)
			updated, cmd := m.picker.list.Update(msg)
			m.picker.list = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	return m, tea.Batch(cmds...)
}

// openEditor suspends the TUI and opens a file in $EDITOR via tea.ExecProcess.
func (m Model) openEditor(path string, onSave func() error, cmds []tea.Cmd) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	c := exec.CommandContext(context.Background(), editor, path)
	cmds = append(cmds, tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err, onSave: onSave}
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

func (m *Model) recordHistory(val string) {
	if strings.TrimSpace(val) == "" {
		return
	}
	if len(m.history) == 0 || m.history[len(m.history)-1] != val {
		m.history = append(m.history, val)
	}
	m.historyIndex = len(m.history)
	m.historyDraft = ""
}

func (m *Model) navigateHistory(delta int) bool {
	if len(m.history) == 0 {
		return false
	}
	if m.historyIndex == 0 && delta < 0 {
		return false
	}
	if m.historyIndex == len(m.history) && delta > 0 {
		return false
	}

	if m.historyIndex == len(m.history) {
		m.historyDraft = m.input.Value()
	}

	m.historyIndex += delta
	if m.historyIndex < 0 {
		m.historyIndex = 0
	}
	if m.historyIndex > len(m.history) {
		m.historyIndex = len(m.history)
	}

	if m.historyIndex == len(m.history) {
		m.input.SetValue(m.historyDraft)
		m.input.CursorEnd()
		return true
	}

	m.input.SetValue(m.history[m.historyIndex])
	m.input.CursorEnd()
	return true
}

// View implements tea.Model.
func (m Model) View() tea.View {
	if !m.ready {
		return tea.NewView("\n  Initializing…")
	}

	// ---- middle: picker or suggestions ----
	var mid strings.Builder
	if m.picker != nil {
		ph := m.height / 2
		ph = max(ph, 10)
		mid.WriteString(m.picker.render(ph) + "\n")
	} else if m.settingsOpen {
		mid.WriteString(m.renderSettingsDialog(max(m.height/2, 10)) + "\n")
	} else if len(m.suggestions) > 0 {
		mid.WriteString(m.renderSuggestions(m.suggestionMaxHeight()))
	}

	helperWidth := max(m.width-2, 0)
	m.help.SetWidth(helperWidth)
	var helpBlock string
	if m.help.ShowAll {
		helpBlock = "\n" + m.help.View(m.helpKeys) + "\n"
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
	midStr := mid.String()
	separatorHeight := 1 // newline between viewport and mid
	fixedHeight := separatorHeight + lipgloss.Height(midStr) + lipgloss.Height(errBlock) + lipgloss.Height(helpBlock) + 2 + lipgloss.Height(footer)
	desiredViewportHeight := max(1, m.height-fixedHeight)
	if m.viewport.Height() != desiredViewportHeight {
		m.viewport.SetHeight(desiredViewportHeight)
	}

	view := tea.NewView(m.viewport.View() + "\n" +
		midStr +
		errBlock +
		helpBlock +
		inputDivider + "\n" +
		inputLine + "\n" +
		footer)
	view.AltScreen = true
	return view
}

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

func formatBackupTimestamp(ts string) string {
	parsed, err := time.Parse("20060102T150405Z", ts)
	if err != nil {
		return ts
	}
	return parsed.UTC().Format("2006-01-02 15:04:05 UTC")
}

// suggestionMaxHeight returns the maximum number of rows available for the suggestion list.
func (m *Model) suggestionMaxHeight() int {
	if m.height <= 0 {
		return 6
	}
	// header(1) + divider(1) + inputDivider(1) + inputLine(1) + footer(1) = 5
	maxRows := m.height - 5
	maxRows = min(max(maxRows, 3), 8)
	return maxRows
}

// renderSuggestions draws the suggestion list with cursor highlighted and windowed.
func (m *Model) renderSuggestions(maxRows int) string {
	list := m.suggestions
	start, end := suggestionWindow(len(list), m.suggCursor, maxRows)

	var sb strings.Builder
	if len(list) == 0 {
		return sb.String()
	}
	for i := start; i < end; i++ {
		s := list[i]
		line := fmt.Sprintf("  /%s  — %s", s.CommandPath, s.Hint)
		if i == m.suggCursor {
			sb.WriteString(suggSelectStyle.Render(line) + "\n")
		} else {
			sb.WriteString(suggNormalStyle.Render(line) + "\n")
		}
	}
	if len(list) > maxRows {
		more := len(list) - (end - start)
		sb.WriteString(suggNormalStyle.Render(fmt.Sprintf("  … %d more", more)) + "\n")
	}
	return sb.String()
}

func (m *Model) renderSettingsDialog(height int) string {
	if m.settingsMenu == nil {
		return ""
	}
	if m.settingsEditing {
		return m.renderSettingsEditor(height)
	}
	return m.renderSettingsList(height)
}

func (m *Model) renderSettingsList(height int) string {
	entries := make([]SettingsEntry, len(m.settingsMenu.Entries))
	copy(entries, m.settingsMenu.Entries)
	SortSettingsEntries(entries)

	items := make([]list.Item, len(entries))
	for i, entry := range entries {
		items[i] = settingsItem{entry: entry}
	}
	m.settingsList.SetItems(items)
	m.settingsList.SetHeight(max(height-2, 4))
	m.settingsList.SetWidth(max(m.width-6, 20))
	m.settingsList.Title = settingsHelpText

	return pickerBorderStyle.Render(m.settingsList.View())
}

type settingsItem struct {
	entry SettingsEntry
}

func (s settingsItem) Title() string { return s.entry.Label }
func (s settingsItem) Description() string {
	if s.entry.Kind == SettingsEntrySubmenu {
		return "submenu"
	}
	return s.entry.Value
}
func (s settingsItem) FilterValue() string { return s.entry.Label }

type settingsDelegate struct{}

func (d settingsDelegate) Height() int  { return 1 }
func (d settingsDelegate) Spacing() int { return 0 }
func (d settingsDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d settingsDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(settingsItem)
	if !ok {
		return
	}
	indicator := " "
	style := pickerNormalStyle
	if index == m.Index() {
		indicator = "›"
		style = pickerCursorStyle
	}

	label := it.entry.Label
	if it.entry.Kind == SettingsEntrySubmenu {
		label = label + " ›"
	}
	if it.entry.Kind == SettingsEntryValue && it.entry.Value != "" {
		label = label + ": " + it.entry.Value
	}
	line := "  " + indicator + " " + label
	_, _ = fmt.Fprint(w, style.Render(fitLine(line, m.Width())))
}

func newSettingsList(width int) list.Model {
	delegate := settingsDelegate{}
	l := list.New([]list.Item{}, delegate, width, 0)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	return l
}

func newSettingsEditor() textarea.Model {
	editor := textarea.New()
	editor.ShowLineNumbers = false
	editor.Prompt = "  "
	editor.SetHeight(5)
	editor.CharLimit = 8000
	styles := editor.Styles()
	styles.Cursor = cursorStyleDef
	editor.SetStyles(styles)
	editor.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("alt+enter"),
		key.WithHelp("alt+enter", "insert newline"),
	)
	return editor
}

func parsePositiveInt(val string) (int, error) {
	if val == "" {
		return 0, errors.New("value must be a positive number")
	}
	parsed, err := strconv.Atoi(val)
	if err != nil || parsed <= 0 {
		return 0, errors.New("value must be a positive number")
	}
	return parsed, nil
}

func (m *Model) handleSettingsEnter() (tea.Model, tea.Cmd) {
	if m.settingsMenu == nil {
		return m, nil
	}
	entry := m.selectedSettingsEntry()
	if entry == nil {
		return m, nil
	}
	if entry.Kind == SettingsEntrySubmenu {
		return m, nil
	}
	if entry.Kind == SettingsEntryValue {
		m.settingsEditing = true
		m.settingsEditEntry = entry
		m.settingsErr = ""
		m.settingsEditor = newSettingsEditor()
		m.settingsEditor.SetValue(entry.Value)
		return m, nil
	}
	return m, nil
}

func (m *Model) handleSettingsSave() (tea.Model, tea.Cmd) {
	if m.settingsEditEntry == nil {
		m.settingsEditing = false
		return m, nil
	}
	val := strings.TrimSpace(m.settingsEditor.Value())
	entry := *m.settingsEditEntry
	if entry.ValueType == SettingsValueNumber {
		parsed, err := parsePositiveInt(val)
		if err != nil {
			m.settingsErr = err.Error()
			return m, nil
		}
		m.memoryTokenBudget = parsed
		entry.Value = fmt.Sprintf("%d", parsed)
		if err := profile.WriteSettings(m.execCtx.ProfileSlug, &profile.Settings{MemoryTokenBudget: parsed}); err != nil {
			m.settingsErr = err.Error()
			return m, nil
		}
	}
	if entry.ValueType == SettingsValueText {
		m.systemPrompt = val
		entry.Value = val
		if err := m.saveSystemPrompt(val); err != nil {
			m.settingsErr = err.Error()
			return m, nil
		}
	}
	m.updateSettingsEntry(entry)
	m.settingsEditing = false
	m.settingsEditEntry = nil
	m.settingsErr = ""
	return m, nil
}

func (m *Model) selectedSettingsEntry() *SettingsEntry {
	item := m.settingsList.SelectedItem()
	if item == nil {
		return nil
	}
	settingsItem, ok := item.(settingsItem)
	if !ok {
		return nil
	}
	entry := settingsItem.entry
	return &entry
}

func (m *Model) updateSettingsEntry(updated SettingsEntry) {
	if m.settingsMenu == nil {
		return
	}
	entries := make([]SettingsEntry, len(m.settingsMenu.Entries))
	copy(entries, m.settingsMenu.Entries)
	for i, entry := range entries {
		if entry.ID == updated.ID {
			entries[i] = updated
			break
		}
	}
	m.settingsMenu.Entries = entries
}

func (m *Model) refreshSettingsValues() {
	if m.settingsMenu == nil || m.execCtx == nil {
		return
	}
	if m.execCtx.ProfileSlug != "" {
		if settings, err := profile.ReadSettings(m.execCtx.ProfileSlug); err == nil {
			m.memoryTokenBudget = settings.MemoryTokenBudget
		}
	}
	if m.execCtx.ProfileID != "" && m.execCtx.DB != nil {
		repo := store.NewSystemPromptRepo(m.execCtx.DB)
		promptStore := profile.NewPromptStore(m.execCtx.ProfileID, repo)
		if prompt, err := promptStore.GetSystemPrompt(context.Background()); err == nil {
			m.systemPrompt = prompt
		}
	}
	m.applySettingsValues()
}

func (m *Model) applySettingsValues() {
	if m.settingsMenu == nil {
		return
	}
	entries := make([]SettingsEntry, len(m.settingsMenu.Entries))
	copy(entries, m.settingsMenu.Entries)
	for i := range entries {
		switch entries[i].ID {
		case settingsIDMemoryTokenLimit:
			if m.memoryTokenBudget > 0 {
				entries[i].Value = fmt.Sprintf("%d", m.memoryTokenBudget)
			}
		case settingsIDSystemPrompt:
			entries[i].Value = m.systemPrompt
		}
	}
	m.settingsMenu.Entries = entries
}

func (m *Model) saveSystemPrompt(prompt string) error {
	if m.execCtx == nil || m.execCtx.DB == nil {
		return errors.New("settings: no database available")
	}
	repo := store.NewSystemPromptRepo(m.execCtx.DB)
	promptStore := profile.NewPromptStore(m.execCtx.ProfileID, repo)
	if err := promptStore.SetSystemPrompt(context.Background(), prompt); err != nil {
		return err
	}
	if m.execCtx.OnPromptChanged != nil {
		return m.execCtx.OnPromptChanged(m.execCtx.ProfileSlug)
	}
	return nil
}

func (m *Model) renderSettingsEditor(height int) string {
	if m.settingsEditEntry == nil {
		return ""
	}
	height = max(height-4, 6)
	m.settingsEditor.SetHeight(height)
	m.settingsEditor.SetWidth(max(m.width-8, 40))
	m.settingsEditor.Focus()
	m.settingsEditor.Prompt = "  "
	m.settingsEditor.SetValue(m.settingsEditor.Value())

	header := lipgloss.NewStyle().Bold(true).Render(m.settingsEditEntry.Label)
	body := m.settingsEditor.View()
	var errBlock string
	if m.settingsErr != "" {
		errBlock = "\n" + errStyle.Render("  "+m.settingsErr)
	}
	return pickerBorderStyle.Render(header + "\n" + body + errBlock)
}

func suggestionWindow(total, cursor, maxRows int) (start, end int) {
	if total == 0 {
		return 0, 0
	}
	if maxRows < 3 {
		maxRows = 3
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}

	start = max(cursor-maxRows/2, 0)
	end = start + maxRows
	if end > total {
		end = total
		start = max(end-maxRows, 0)
	}
	return start, end
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

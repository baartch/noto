package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
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

	"noto/internal/cache"
	"noto/internal/chat"
	"noto/internal/commands"
	"noto/internal/profile"
	"noto/internal/provider"
	"noto/internal/security"
	"noto/internal/store"
	"noto/internal/suggest"
)

// ---- callback types ---------------------------------------------------------

// ProviderFunc sends a single chat turn and returns the assistant reply.
type ProviderFunc func(ctx context.Context, userMsg string) (string, error)

// ListModelsFunc fetches the available models from the configured provider.
type ListModelsFunc func(ctx context.Context) ([]provider.ModelInfo, error)

// ListEmbeddingsFunc fetches available embeddings models from the provider.
type ListEmbeddingsFunc func(ctx context.Context) ([]provider.ModelInfo, error)

// ModelSelectedFunc is called when the user picks a model in the picker.
type ModelSelectedFunc func(modelID string) error

// EmbeddingModelSelectedFunc is called when the user picks an embeddings model.
type EmbeddingModelSelectedFunc func(modelID string) error

// ProfileSwitchCmd switches the active profile and refreshes the TUI.
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

// CacheStatusUpdated updates the ctx status badge in the footer.
func CacheStatusUpdated(formatted string) tea.Msg { return cacheStatusUpdatedMsg{formatted: formatted} }

// UpdateAvailableNotice shows a transient update warning in the footer.
func UpdateAvailableNotice(formatted string) tea.Msg { return updateNoticeMsg{formatted: formatted} }

// ProfileSwitched updates the TUI state after switching profiles.
func ProfileSwitched(profileName, activeModel, extractorModel, embeddingModel, cacheStatus, tokenStatus string, extractorFallback, embeddingModelMissing bool, provider ProviderFunc, listModels ListModelsFunc, listEmbeddings ListEmbeddingsFunc, modelSelected ModelSelectedFunc, embeddingModelSelected EmbeddingModelSelectedFunc, extractorModelSelected ExtractorModelSelectedFunc, settings *SettingsMenu, history []string, startupMessages []*store.Message, startupHistoryErr error) profileSwitchedMsg {
	return profileSwitchedMsg{
		profileName:            profileName,
		activeModel:            activeModel,
		extractorModel:         extractorModel,
		embeddingModel:         embeddingModel,
		cacheStatus:            cacheStatus,
		tokenStatus:            tokenStatus,
		extractorFallback:      extractorFallback,
		embeddingModelMissing:  embeddingModelMissing,
		provider:               provider,
		listModels:             listModels,
		listEmbeddings:         listEmbeddings,
		modelSelected:          modelSelected,
		embeddingModelSelected: embeddingModelSelected,
		extractorModelSelected: extractorModelSelected,
		settings:               settings,
		history:                history,
		startupMessages:        startupMessages,
		startupHistoryErr:      startupHistoryErr,
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
type cacheStatusUpdatedMsg struct{ formatted string }
type updateNoticeMsg struct{ formatted string }

type profilesAction interface {
	Label() string
	Apply(ctx context.Context, svc *profile.Service, selected string) (*store.Profile, error)
}

type profilesCreateAction struct{}

type profilesRenameAction struct {
	current string
}

func (profilesCreateAction) Label() string { return "New Profile" }

func (profilesCreateAction) Apply(ctx context.Context, svc *profile.Service, value string) (*store.Profile, error) {
	if svc == nil {
		return nil, errors.New("settings: no profile service")
	}
	return svc.Create(ctx, value)
}

func (a profilesRenameAction) Label() string {
	return "Rename Profile"
}

func (a profilesRenameAction) Apply(ctx context.Context, svc *profile.Service, value string) (*store.Profile, error) {
	if svc == nil {
		return nil, errors.New("settings: no profile service")
	}
	return svc.Rename(ctx, a.current, value)
}

type profileSwitchedMsg struct {
	profileName            string
	activeModel            string
	extractorModel         string
	embeddingModel         string
	cacheStatus            string
	tokenStatus            string
	extractorFallback      bool
	embeddingModelMissing  bool
	provider               ProviderFunc
	listModels             ListModelsFunc
	listEmbeddings         ListEmbeddingsFunc
	modelSelected          ModelSelectedFunc
	embeddingModelSelected EmbeddingModelSelectedFunc
	extractorModelSelected ExtractorModelSelectedFunc
	settings               *SettingsMenu
	history                []string
	startupMessages        []*store.Message
	startupHistoryErr      error
}
type profileSwitchFailedMsg struct{ err error }
type spinnerTickMsg struct{}

// ---- picker kind ------------------------------------------------------------

type pickerKind int

const (
	pickerKindModel pickerKind = iota
	pickerKindBackup
	pickerKindExtractorModel
	pickerKindEmbeddingsModel
)

const (
	settingsHeaderText = "Settings"
	settingsHelpText   = "  Settings"
)

// ---- TUI model --------------------------------------------------------------

type scrollZone int

const (
	scrollZoneOutside scrollZone = iota
	scrollZoneMessages
	scrollZoneInput
)

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
	historyErr  string
	ready       bool
	keys        keyMap
	help        help.Model
	helpKeys    helpKeyMap

	// slash suggestion state
	suggestions []suggest.Suggestion
	suggCursor  int
	suggActive  bool

	// input history
	history         []string
	historyAll      []string
	historyIndex    int
	historyDraft    string
	historyBaseFrom int

	conversationHistoryWindow conversationHistoryWindow
	inputHistoryWindow        inputHistoryWindow
	allMessages               []chatMessage
	messageLoadInProgress     bool

	// picker overlay
	picker             *pickerState
	pickerKind         pickerKind
	pickerFromSettings bool

	// settings dialog
	settingsOpen bool
	settingsMenu *SettingsMenu

	// settings editor
	settingsList      list.Model
	settingsEditing   bool
	settingsEditor    textarea.Model
	settingsEditEntry *SettingsEntry
	settingsErr       string

	profileDeleteCandidate string

	// profile settings
	rawNoteDays          int
	weeklySummaryWeeks   int
	monthlySummaryMonths string
	systemPrompt         string
	providerEndpoint     string
	providerAPIKey       string // decrypted; displayed obfuscated

	// notes badge
	notesIndicator string

	updateNotice string

	// pending assistant state
	pending      bool
	spinnerIndex int

	dispatcher             *chat.Dispatcher
	execCtx                *commands.ExecContext
	provider               ProviderFunc
	listModels             ListModelsFunc
	listEmbeddings         ListEmbeddingsFunc
	modelSelected          ModelSelectedFunc
	embeddingModelSelected EmbeddingModelSelectedFunc
	profileService         *profile.Service
	profileSwitch          ProfileSwitchCmd
	listBackups            ListBackupsFunc
	backupSelected         BackupSelectedFunc
	profilesAction         profilesAction
	extractorModel         string
	extractorModelSelected ExtractorModelSelectedFunc
	extractorFallback      bool
	embeddingModelMissing  bool
	promptBootstrapWarning bool
	embeddingModel         string

	sidebar *sidebarModel
}

type conversationHistoryWindow struct {
	messages  []chatMessage
	hasOlder  bool
	loadedCnt int
}

type inputHistoryWindow struct {
	entries   []string
	hasOlder  bool
	loadedCnt int
}

type chatMessage struct {
	role                  string
	content               string
	timestamp             time.Time
	conversationID        string
	conversationStartedAt time.Time
}

type keyMap struct {
	quit          key.Binding
	openModel     key.Binding
	clearInput    key.Binding
	toggleHelp    key.Binding
	openSettings  key.Binding
	toggleSidebar key.Binding
	profileNew    key.Binding
	profileRename key.Binding
	profileDelete key.Binding
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
	embeddingModel string,
	cacheStatus string,
	tokenStatus string,
	extractorFallback bool,
	embeddingModelMissing bool,
	dispatcher *chat.Dispatcher,
	execCtx *commands.ExecContext,
	providerFn ProviderFunc,
	listModels ListModelsFunc,
	listEmbeddings ListEmbeddingsFunc,
	modelSelected ModelSelectedFunc,
	embeddingModelSelected EmbeddingModelSelectedFunc,
	profileService *profile.Service,
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
	ti.KeyMap.WordForward = key.NewBinding(
		key.WithKeys("alt+right", "alt+f", "ctrl+right"),
		key.WithHelp("ctrl+right", "word forward"),
	)
	ti.KeyMap.WordBackward = key.NewBinding(
		key.WithKeys("alt+left", "alt+b", "ctrl+left"),
		key.WithHelp("ctrl+left", "word backward"),
	)
	keys := keyMap{
		quit:          key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "quit")),
		openModel:     key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "model picker")),
		clearInput:    key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "clear")),
		toggleHelp:    key.NewBinding(key.WithKeys("ctrl+h"), key.WithHelp("ctrl+h", "help")),
		openSettings:  key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "settings")),
		toggleSidebar: key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "sidebar")),
		profileNew:    key.NewBinding(key.WithKeys("ctrl+n"), key.WithHelp("ctrl+n", "new profile")),
		profileRename: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "rename profile")),
		profileDelete: key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("ctrl+k", "delete profile")),
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
			keys.toggleSidebar,
			keys.clearInput,
			key.NewBinding(key.WithKeys("wheel"), key.WithHelp("wheel", "scroll messages/input by cursor zone")),
			key.NewBinding(key.WithKeys("pgup/pgdn"), key.WithHelp("pgup/pgdn", "scroll messages")),
			key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "jump to latest message")),
			key.NewBinding(key.WithKeys("shift+click"), key.WithHelp("shift+click", "select text to copy")),
			ti.KeyMap.InsertNewline,
			ti.KeyMap.WordBackward,
			ti.KeyMap.WordForward,
			keys.quit,
		},
	}

	if inputHistory == nil {
		inputHistory = []string{}
	}

	m := Model{
		profileName:            profileName,
		activeModel:            activeModel,
		cacheStatus:            cacheStatus,
		tokenStatus:            tokenStatus,
		input:                  ti,
		suggCursor:             -1,
		historyAll:             append([]string(nil), inputHistory...),
		historyIndex:           0,
		dispatcher:             dispatcher,
		execCtx:                execCtx,
		provider:               providerFn,
		embeddingModelMissing:  embeddingModelMissing,
		listEmbeddings:         listEmbeddings,
		embeddingModelSelected: embeddingModelSelected,
		embeddingModel:         embeddingModel,
		keys:                   keys,
		help:                   helpModel,
		helpKeys:               helpKeys,
		listModels:             listModels,
		modelSelected:          modelSelected,
		profileService:         profileService,
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
		historyErr:             "",
		sidebar: func() *sidebarModel {
			if execCtx == nil || execCtx.DB == nil {
				return nil
			}
			return newSidebar(36,
				store.NewMemoryNoteRepo(execCtx.DB),
				store.NewMemorySummaryRepo(execCtx.DB),
				execCtx.ProfileID)
		}(),
	}
	m.loadInitialConversationHistory(nil)
	m.resetInputHistoryWindow()
	return m
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
		if m.sidebar != nil {
			m.sidebar.width = max(msg.Width/5, 36)
		}
		mainWidth := m.width
		if m.sidebar != nil && m.sidebar.open {
			mainWidth = m.width - m.sidebar.width - 1
		}
		m.input.SetWidth(mainWidth - 4)
		// header(1) + divider(1) + inputDivider(1) + inputLine(1) + padding(1) = 5
		vpH := msg.Height - 5 // inputDivider+input+hint+footer
		vpH = max(vpH, 1)
		if !m.ready {
			m.viewport = viewport.New(viewport.WithWidth(mainWidth), viewport.WithHeight(vpH))
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			m.ready = true
		} else {
			m.viewport.SetWidth(mainWidth)
			m.viewport.SetHeight(vpH)
		}

	// ---- clipboard ----------------------------------------------------------
	case tea.PasteMsg:
		if m.settingsOpen && m.settingsEditing {
			var cmd tea.Cmd
			m.settingsEditor, cmd = m.settingsEditor.Update(msg)
			return m, cmd
		}

	// ---- mouse --------------------------------------------------------------
	case tea.MouseWheelMsg:
		if m.sidebar != nil && m.sidebar.open {
			mouse := msg.Mouse()
			if mouse.X >= m.width-m.sidebar.width {
				return m.handleSidebarMouseWheel(msg)
			}
		}
		return m.handleMouseWheel(msg)

	// ---- keyboard -----------------------------------------------------------
	case tea.KeyPressMsg:
		if m.picker != nil {
			return m.updatePicker(msg, cmds)
		}
		if m.settingsOpen && m.settingsEditing {
			// ctrl+h toggles help even while editing
			if key.Matches(msg, m.keys.toggleHelp) {
				m.help.ShowAll = !m.help.ShowAll
				return m, nil
			}
			switch {
			case msg.Key().Code == tea.KeyEsc:
				m.settingsEditing = false
				m.settingsEditEntry = nil
				m.settingsErr = ""
				m.profileDeleteCandidate = ""
				m.profilesAction = nil
				return m, nil
			// enter saves; alt+enter falls through to the textarea for newline
			case msg.Key().Code == tea.KeyEnter && msg.Key().Mod == 0:
				return m.handleSettingsSave()
			}
			var cmd tea.Cmd
			m.settingsEditor, cmd = m.settingsEditor.Update(msg)
			return m, cmd
		}
		if m.settingsOpen && !m.settingsEditing {
			if m.profileDeleteCandidate != "" {
				switch msg.Key().Code {
				case tea.KeyEnter:
					return m.confirmProfileDelete()
				case tea.KeyEsc:
					m.profileDeleteCandidate = ""
					m.settingsErr = ""
					return m, nil
				default:
					m.profileDeleteCandidate = ""
					m.settingsErr = ""
					return m, nil
				}
			}
			// ctrl+h toggles help; ctrl+j closes settings
			if key.Matches(msg, m.keys.toggleHelp) {
				m.help.ShowAll = !m.help.ShowAll
				return m, nil
			}
			if key.Matches(msg, m.keys.openSettings) || msg.String() == "ctrl+j" || msg.Key().Keystroke() == "ctrl+j" {
				m.settingsOpen = false
				m.settingsErr = ""
				return m, m.input.Focus()
			}
			if m.settingsMenu != nil && m.settingsMenu.ID == settingsIDProfiles {
				switch {
				case key.Matches(msg, m.keys.profileNew):
					return m.startProfileEditor("New Profile", profilesCreateAction{})
				case key.Matches(msg, m.keys.profileRename):
					return m.startProfileEditor("Rename Profile", profilesRenameAction{current: m.selectedProfileName()})
				case key.Matches(msg, m.keys.profileDelete):
					return m.handleProfileDeleteRequest()
				}
			}
			switch msg.Key().Code {
			case tea.KeyEsc:
				if next, ok := NavigateUp(m.settingsMenu); ok {
					m.settingsMenu = next
					m.syncSettingsList()
					return m, nil
				}
				m.settingsOpen = false
				m.settingsErr = ""
				m.profileDeleteCandidate = ""
				return m, m.input.Focus()
			case tea.KeyEnter:
				return m.handleSettingsEnter()
			default:
				var cmd tea.Cmd
				m.settingsList, cmd = m.settingsList.Update(msg)
				return m, cmd
			}
		}
		if m.suggActive && len(m.suggestions) > 0 {
			return m.updateSuggNav(msg, cmds)
		}

		// Sidebar: Tab/Shift+Tab cycle tabs, Esc closes.
		if m.sidebar != nil && m.sidebar.open {
			switch {
			case key.Matches(msg, m.keys.toggleSidebar):
				return m.toggleSidebar()
			case msg.Key().Code == tea.KeyTab:
				m.sidebar.activeTab = (m.sidebar.activeTab + 1) % 3
				m.sidebar.viewport.SetContent(m.sidebar.renderContent())
				m.sidebar.viewport.GotoBottom()
				return m, nil
			case msg.Key().Code == tea.KeyTab && msg.Key().Mod.Contains(tea.ModShift):
				m.sidebar.activeTab = (m.sidebar.activeTab + 2) % 3
				m.sidebar.viewport.SetContent(m.sidebar.renderContent())
				m.sidebar.viewport.GotoBottom()
				return m, nil
			case msg.Key().Code == tea.KeyEsc:
				return m.toggleSidebar()
			}
			// Other keys fall through to normal handling.
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

		case key.Matches(msg, m.keys.toggleSidebar):
			return m.toggleSidebar()

		case key.Matches(msg, m.keys.openSettings) || msg.String() == "ctrl+j" || msg.Key().Keystroke() == "ctrl+j":
			m.clearSuggestions()
			m.input.SetValue("")
			m.settingsOpen = !m.settingsOpen
			if m.settingsOpen {
				m.settingsEditing = false
				m.settingsErr = ""
				m.profileDeleteCandidate = ""
				m.profilesAction = nil
				if m.settingsMenu == nil {
					m.settingsMenu = DefaultSettingsMenu()
				}
				m.input.Blur()
				m.refreshSettingsValues()
				m.syncSettingsList()
			} else {
				m.input.Focus()
			}
			return m, nil

		case msg.Key().Code == tea.KeyEsc:
			if m.settingsOpen {
				if !m.settingsEditing {
					if next, ok := NavigateUp(m.settingsMenu); ok {
						m.settingsMenu = next
						return m, nil
					}
				}
				m.settingsOpen = false
				m.settingsEditing = false
				m.settingsErr = ""
				m.profileDeleteCandidate = ""
				m.profilesAction = nil
				return m, nil
			}
			if len(m.suggestions) > 0 {
				m.clearSuggestions()
				m.input.SetValue("")
				return m, nil
			}
			return m, tea.Quit

		case msg.Key().Code == tea.KeyUp:
			if len(m.suggestions) > 0 {
				m.suggActive = true
				m.suggCursor = len(m.suggestions) - 1
				m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
				m.input.CursorEnd()
				return m, nil
			}
			m.ensureInputHistoryLoaded()
			if m.navigateHistory(-1) {
				return m, nil
			}
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd

		case msg.Key().Code == tea.KeyDown:
			if len(m.suggestions) > 0 {
				m.suggActive = true
				m.suggCursor = 0
				m.input.SetValue("/" + m.suggestions[m.suggCursor].CommandPath)
				m.input.CursorEnd()
				return m, nil
			}
			m.ensureInputHistoryLoaded()
			if m.navigateHistory(1) {
				return m, nil
			}
			var vpCmd tea.Cmd
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, vpCmd

		case msg.Key().Code == tea.KeyPgUp:
			var vpCmd tea.Cmd
			for range conversationWheelStepLines {
				m.viewport, vpCmd = m.viewport.Update(tea.KeyPressMsg{Code: tea.KeyUp})
			}
			m.maybeLoadOlderConversationHistory()
			return m, vpCmd

		case msg.Key().Code == tea.KeyPgDown:
			var vpCmd tea.Cmd
			for range conversationWheelStepLines {
				m.viewport, vpCmd = m.viewport.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			}
			return m, vpCmd

		case msg.Key().Code == tea.KeyEnd:
			m.viewport.GotoBottom()
			m.input.CursorEnd()
			return m, nil

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

		case msg.Key().Code == tea.KeyEnter && msg.Key().Mod == 0:
			// Send on Enter (newline handled by textarea via Alt+Enter)
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.clearSuggestions()
			m.err = nil
			m.recordHistory(val)
			m.resetInputHistoryWindow()
			return m.handleSubmit(val, cmds)
		}

	// ---- picker items loaded ------------------------------------------------
	case modelsLoadedMsg:
		if m.picker != nil && (m.pickerKind == pickerKindModel || m.pickerKind == pickerKindExtractorModel || m.pickerKind == pickerKindEmbeddingsModel) {
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
		m.updateNotice = ""
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
		m.updateNotice = ""

	case clearNotesIndicatorMsg:
		m.notesIndicator = ""

	// ---- sidebar ------------------------------------------------------------
	case sidebarBatchMsg:
		oldContent := m.sidebar.renderContent()
		oldHeight := lipgloss.Height(oldContent)
		wasAtBottom := m.sidebar.viewport.AtBottom()
		switch msg.tab {
		case sidebarTabNotes:
			m.sidebar.notes = append(m.sidebar.notes, msg.notes...)
		case sidebarTabWeekly:
			m.sidebar.weekly = append(m.sidebar.weekly, msg.summaries...)
		case sidebarTabMonthly:
			m.sidebar.monthly = append(m.sidebar.monthly, msg.summaries...)
		}
		m.sidebar.hasMore = msg.hasMore
		m.sidebar.loading = false
		newContent := m.sidebar.renderContent()
		newHeight := lipgloss.Height(newContent)
		m.sidebar.viewport.SetContent(newContent)
		if wasAtBottom {
			m.sidebar.viewport.GotoBottom()
		} else {
			m.sidebar.viewport.SetYOffset(m.sidebar.viewport.YOffset() + newHeight - oldHeight)
		}

	case sidebarLoadErr:
		m.sidebar.loading = false
		m.err = msg.err

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

	case cacheStatusUpdatedMsg:
		m.cacheStatus = msg.formatted

	case updateNoticeMsg:
		m.updateNotice = msg.formatted

	case profileSwitchedMsg:
		m.profileName = msg.profileName
		m.activeModel = msg.activeModel
		m.extractorModel = msg.extractorModel
		m.extractorFallback = msg.extractorFallback
		m.embeddingModelMissing = msg.embeddingModelMissing
		m.cacheStatus = msg.cacheStatus
		m.tokenStatus = msg.tokenStatus
		m.provider = msg.provider
		m.listModels = msg.listModels
		m.listEmbeddings = msg.listEmbeddings
		m.modelSelected = msg.modelSelected
		m.embeddingModelSelected = msg.embeddingModelSelected
		m.extractorModelSelected = msg.extractorModelSelected
		m.embeddingModel = msg.embeddingModel
		m.settingsMenu = msg.settings
		m.rawNoteDays = 0
		m.weeklySummaryWeeks = 0
		m.monthlySummaryMonths = ""
		m.systemPrompt = ""
		m.settingsEditEntry = nil
		m.settingsEditing = false
		m.settingsErr = ""
		m.historyAll = append([]string(nil), msg.history...)
		m.historyDraft = ""
		m.resetInputHistoryWindow()
		m.messages = []chatMessage{{role: "command", content: "Switched to profile: " + msg.profileName, timestamp: time.Now()}}
		if len(msg.startupMessages) > 0 {
			mapped := m.mapStoreMessagesToChatMessages(msg.startupMessages)
			m.loadInitialConversationHistory(mapped)
			m.messages = m.conversationHistoryWindow.messages
		} else {
			m.loadInitialConversationHistory(nil)
		}
		m.setHistoryError(msg.startupHistoryErr)
		m.err = nil
		m.clearSuggestions()
		m.input.SetValue("")
		m.refreshSettingsValues()
		m.syncSettingsList()
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

func (m *Model) loadInitialConversationHistory(messages []chatMessage) {
	if len(messages) == 0 {
		m.conversationHistoryWindow = conversationHistoryWindow{messages: m.messages, hasOlder: false, loadedCnt: len(m.messages)}
		m.allMessages = append([]chatMessage(nil), m.messages...)
		return
	}
	m.allMessages = append([]chatMessage(nil), messages...)
	if len(messages) > conversationInitialLoadSize {
		messages = messages[len(messages)-conversationInitialLoadSize:]
		m.conversationHistoryWindow.hasOlder = true
	} else {
		// For profile-wide startup loads we only fetch the latest window first.
		// If window is full-size, optimistically allow one older fetch attempt.
		m.conversationHistoryWindow.hasOlder = len(messages) == conversationInitialLoadSize
	}
	m.conversationHistoryWindow.messages = append([]chatMessage(nil), messages...)
	m.conversationHistoryWindow.loadedCnt = len(messages)
}

func (m *Model) loadOlderConversationHistoryBatch() {
	if m.messageLoadInProgress || !m.conversationHistoryWindow.hasOlder {
		return
	}
	m.messageLoadInProgress = true
	defer func() { m.messageLoadInProgress = false }()

	loaded := len(m.conversationHistoryWindow.messages)
	total := len(m.allMessages)
	if loaded >= total {
		m.conversationHistoryWindow.hasOlder = false
		return
	}
	remaining := total - loaded
	batch := min(conversationLazyBatchSize, remaining)
	start := max(total-loaded-batch, 0)
	older := m.allMessages[start : start+batch]

	anchor := m.viewport.YOffset()
	beforeLines := m.viewport.TotalLineCount()
	combined := make([]chatMessage, 0, len(older)+len(m.conversationHistoryWindow.messages))
	combined = append(combined, older...)
	combined = append(combined, m.conversationHistoryWindow.messages...)
	m.conversationHistoryWindow.messages = combined
	m.conversationHistoryWindow.loadedCnt = len(combined)
	m.conversationHistoryWindow.hasOlder = len(combined) < total
	m.messages = combined
	if m.ready {
		m.viewport.SetContent(m.renderHistory())
		afterLines := m.viewport.TotalLineCount()
		m.viewport.SetYOffset(anchor + max(0, afterLines-beforeLines))
	}
}

func (m *Model) resetInputHistoryWindow() {
	m.inputHistoryWindow = inputHistoryWindow{}
	m.history = nil
	m.historyBaseFrom = len(m.historyAll)
	m.historyIndex = 0
}

func (m *Model) loadInitialInputHistoryBatch() {
	if len(m.historyAll) == 0 {
		m.inputHistoryWindow = inputHistoryWindow{}
		m.history = nil
		m.historyBaseFrom = 0
		m.historyIndex = 0
		return
	}
	start := max(len(m.historyAll)-inputFirstLoadSize, 0)
	entries := append([]string(nil), m.historyAll[start:]...)
	m.inputHistoryWindow.entries = entries
	m.inputHistoryWindow.loadedCnt = len(entries)
	m.inputHistoryWindow.hasOlder = start > 0
	m.history = entries
	m.historyBaseFrom = start
	m.historyIndex = len(m.history)
}

func (m *Model) maybeLoadOlderConversationHistory() {
	if !m.ready || !m.viewport.AtTop() {
		return
	}
	if len(m.allMessages) > len(m.conversationHistoryWindow.messages) {
		m.loadOlderConversationHistoryBatch()
		return
	}
	m.loadOlderConversationHistoryFromStore()
}

func (m *Model) loadOlderConversationHistoryFromStore() {
	if m.messageLoadInProgress || !m.conversationHistoryWindow.hasOlder {
		return
	}
	if m.execCtx == nil || m.execCtx.DB == nil || m.execCtx.ProfileID == "" {
		m.conversationHistoryWindow.hasOlder = false
		return
	}
	if len(m.conversationHistoryWindow.messages) == 0 {
		m.conversationHistoryWindow.hasOlder = false
		return
	}
	before := m.conversationHistoryWindow.messages[0].timestamp
	if before.IsZero() {
		m.conversationHistoryWindow.hasOlder = false
		return
	}

	m.messageLoadInProgress = true
	defer func() { m.messageLoadInProgress = false }()

	repo := store.NewMessageRepo(m.execCtx.DB)
	older, err := repo.ListOlderByProfileBefore(context.Background(), m.execCtx.ProfileID, before, conversationLazyBatchSize)
	if err != nil {
		m.setHistoryError(err)
		return
	}
	if len(older) == 0 {
		m.conversationHistoryWindow.hasOlder = false
		return
	}

	mapped := m.mapStoreMessagesToChatMessages(older)
	anchor := m.viewport.YOffset()
	beforeLines := m.viewport.TotalLineCount()
	combined := make([]chatMessage, 0, len(mapped)+len(m.conversationHistoryWindow.messages))
	combined = append(combined, mapped...)
	combined = append(combined, m.conversationHistoryWindow.messages...)
	m.conversationHistoryWindow.messages = combined
	m.conversationHistoryWindow.loadedCnt = len(combined)
	m.conversationHistoryWindow.hasOlder = len(mapped) == conversationLazyBatchSize
	m.messages = combined
	m.setHistoryError(nil)
	if m.ready {
		m.viewport.SetContent(m.renderHistory())
		afterLines := m.viewport.TotalLineCount()
		m.viewport.SetYOffset(anchor + max(0, afterLines-beforeLines))
	}
}

func (m *Model) setHistoryError(err error) {
	if err == nil {
		m.historyErr = ""
		return
	}
	m.historyErr = err.Error()
}

func (m *Model) mapStoreMessagesToChatMessages(messages []*store.Message) []chatMessage {
	out := make([]chatMessage, 0, len(messages))
	for _, sm := range messages {
		if sm == nil {
			continue
		}
		role := "command"
		switch sm.Role {
		case store.RoleUser:
			role = "user"
		case store.RoleAssistant:
			role = "assistant"
		case store.RoleSystem:
			role = "command"
		}
		out = append(out, chatMessage{
			role:                  role,
			content:               sm.Content,
			timestamp:             sm.CreatedAt,
			conversationID:        sm.ConversationID,
			conversationStartedAt: sm.ConversationStartedAt,
		})
	}
	return out
}

func (m *Model) recordHistory(val string) {
	if strings.TrimSpace(val) == "" {
		return
	}
	if len(m.history) == 0 || m.history[len(m.history)-1] != val {
		m.history = append(m.history, val)
	}
	m.historyAll = append(m.historyAll, val)
	m.historyIndex = len(m.history)
	m.historyDraft = ""
	m.inputHistoryWindow.entries = append([]string(nil), m.history...)
	m.inputHistoryWindow.loadedCnt = len(m.inputHistoryWindow.entries)
	m.inputHistoryWindow.hasOlder = m.historyBaseFrom > 0
}

func (m *Model) navigateHistory(delta int) bool {
	if len(m.history) == 0 {
		return false
	}
	if delta < 0 && m.historyIndex == 0 {
		m.loadOlderInputHistoryBatch()
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

func (m *Model) ensureInputHistoryLoaded() {
	if len(m.history) == 0 && len(m.historyAll) > 0 {
		m.loadInitialInputHistoryBatch()
	}
}

func (m *Model) loadOlderInputHistoryBatch() {
	if !m.inputHistoryWindow.hasOlder || len(m.inputHistoryWindow.entries) >= inputHistoryMaxLoaded {
		return
	}
	load := inputLazyBatchSize
	remaining := inputHistoryMaxLoaded - len(m.inputHistoryWindow.entries)
	if load > remaining {
		load = remaining
	}
	start := max(m.historyBaseFrom-load, 0)
	end := m.historyBaseFrom
	if end <= start {
		m.inputHistoryWindow.hasOlder = false
		return
	}
	older := append([]string(nil), m.historyAll[start:end]...)
	m.inputHistoryWindow.entries = append(older, m.inputHistoryWindow.entries...)
	m.inputHistoryWindow.loadedCnt = len(m.inputHistoryWindow.entries)
	m.historyBaseFrom = start
	m.inputHistoryWindow.hasOlder = start > 0 && len(m.inputHistoryWindow.entries) < inputHistoryMaxLoaded
	m.history = append([]string(nil), m.inputHistoryWindow.entries...)
	m.historyIndex = load
}

// View implements tea.Model.
func (m Model) View() tea.View {
	if !m.ready {
		return tea.NewView("")
	}

	mainWidth := m.width
	if m.sidebar != nil && m.sidebar.open {
		mainWidth = m.width - m.sidebar.width - 1
	}

	// ---- middle: picker or suggestions ----
	var mid strings.Builder
	ph := max(m.height/2, 10)
	switch {
	case m.picker != nil:
		mid.WriteString(m.picker.render(ph) + "\n")
	case m.settingsOpen:
		mid.WriteString(m.renderSettingsDialog(ph) + "\n")
	case len(m.suggestions) > 0:
		mid.WriteString(m.renderSuggestions(m.suggestionMaxHeight()))
	}

	helperWidth := max(mainWidth-2, 0)
	m.help.SetWidth(helperWidth)
	m.updateListHelp()
	var helpBlock string
	if m.help.ShowAll && m.picker == nil {
		helpBlock = "\n" + m.help.View(m.helpKeys) + "\n"
	}

	var errBlock string
	if m.err != nil {
		errBlock = errStyle.Render("  ✗ "+m.err.Error()) + "\n"
	}
	if m.historyErr != "" {
		errBlock += errStyle.Render("  ! "+m.historyErr) + "\n"
	}

	// ---- input bar ----
	inputDivider := dividerStyle.Render(strings.Repeat("─", mainWidth))
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

	content := m.viewport.View() + "\n" +
		midStr +
		errBlock +
		helpBlock +
		inputDivider + "\n" +
		inputLine + "\n" +
		footer
	if m.sidebar != nil && m.sidebar.open {
		sidebarContent := m.sidebar.render(m.height)
		mainLines := strings.Split(content, "\n")
		sideLines := strings.Split(sidebarContent, "\n")
		n := max(len(mainLines), len(sideLines))
		var sb strings.Builder
		for i := range n {
			var ml, sl string
			if i < len(mainLines) {
				ml = mainLines[i]
			}
			if i < len(sideLines) {
				sl = sideLines[i]
			}
			w := lipgloss.Width(ml)
			if w < mainWidth {
				ml += strings.Repeat(" ", mainWidth-w)
			}
			sb.WriteString(ml)
			sb.WriteString(sl)
			if i < n-1 {
				sb.WriteString("\n")
			}
		}
		content = sb.String()
	}
	view := tea.NewView(content)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
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
	m.updateListHelp()
	m.settingsList.SetHeight(max(height-2, 4))
	m.settingsList.SetWidth(max(m.width-6, 20))
	m.settingsList.Title = ""

	head := settingsHelpText
	if m.settingsErr != "" {
		head = head + "\n" + errStyle.Render("  "+m.settingsErr)
	}

	return pickerBorderStyle.Render(head + "\n" + m.settingsList.View())
}

func (m *Model) syncSettingsList() {
	if m.settingsMenu == nil {
		return
	}
	entries := make([]SettingsEntry, len(m.settingsMenu.Entries))
	copy(entries, m.settingsMenu.Entries)
	SortSettingsEntries(entries)

	items := make([]list.Item, len(entries))
	for i, entry := range entries {
		items[i] = settingsItem{entry: entry}
	}
	m.settingsList.SetItems(items)
	m.settingsList.Title = ""
	m.settingsList.SetSize(max(m.width-6, 20), max(m.height/2-2, 6))
	if len(items) > 0 {
		m.settingsList.Select(0)
	}
}

func (m *Model) refreshProfilesList() {
	if m.settingsMenu == nil {
		return
	}
	var menu *SettingsMenu
	if m.settingsMenu.ID == settingsIDProfiles {
		menu = m.settingsMenu
	} else if next, ok := NavigateToSubmenu(m.settingsMenu, settingsIDProfiles); ok {
		menu = next
	}
	if menu == nil {
		return
	}
	menu.Entries = nil
	if m.profileService == nil {
		return
	}
	profiles, err := m.profileService.List(context.Background())
	if err != nil {
		return
	}
	items := make([]SettingsEntry, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, SettingsEntry{
			ID:     p.Name,
			Label:  p.Name,
			Kind:   SettingsEntryProfile,
			Value:  p.Slug,
			Active: p.IsDefault,
		})
	}
	menu.Entries = items
}

func (m *Model) selectedProfileName() string {
	entry := m.selectedSettingsEntry()
	if entry == nil {
		return ""
	}
	return entry.ID
}

func (m *Model) selectProfileEntry(entry *SettingsEntry) (tea.Model, tea.Cmd) {
	if entry == nil || m.profileService == nil {
		return m, nil
	}
	if m.profileSwitch != nil {
		return m, m.profileSwitch(entry.ID)
	}
	m.settingsMenu = DefaultSettingsMenu()
	p, err := m.profileService.Select(context.Background(), entry.ID)
	if err != nil {
		m.settingsErr = err.Error()
		return m, nil
	}
	m.profileName = p.Name
	m.execCtx.ProfileID = p.ID
	m.execCtx.ProfileSlug = p.Slug
	m.settingsErr = ""
	m.settingsMenu = DefaultSettingsMenu()
	m.refreshSettingsValues()
	m.syncSettingsList()
	return m, nil
}

func (m *Model) startProfileEditor(label string, action profilesAction) (tea.Model, tea.Cmd) {
	m.settingsEditing = true
	m.settingsErr = ""
	m.settingsEditEntry = &SettingsEntry{ID: settingsIDProfiles, Label: label, Kind: SettingsEntryValue, ValueType: SettingsValueText}
	m.settingsEditor = newSettingsEditor()
	if action != nil {
		m.settingsEditor.SetValue("")
		if renameAction, ok := action.(profilesRenameAction); ok {
			m.settingsEditor.SetValue(renameAction.current)
		}
		m.profilesAction = action
	}
	return m, m.settingsEditor.Focus()
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
	switch {
	case index == m.Index():
		indicator = "›"
		style = pickerCursorStyle
	case it.entry.Active:
		indicator = "●"
		style = pickerActiveStyle
	}

	label := it.entry.Label
	switch it.entry.Kind {
	case SettingsEntrySubmenu:
		label += " ›"
	case SettingsEntryAction:
		label += " →"
		if it.entry.Value != "" {
			label = label + " · " + it.entry.Value
		}
	case SettingsEntryValue:
		if it.entry.Value != "" {
			label = label + ": " + formatSettingsValue(it.entry.Value, m.Width()-10)
		}
	case SettingsEntryProfile:
		if it.entry.Value != "" {
			label = label + " · " + it.entry.Value
		}
	}
	line := "  " + indicator + " " + label
	_, _ = fmt.Fprint(w, style.Render(fitLine(line, m.Width())))
}

func formatSettingsValue(value string, maxWidth int) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	if maxWidth <= 0 {
		return trimmed
	}
	if lipgloss.Width(trimmed) <= maxWidth {
		return trimmed
	}
	if maxWidth <= 3 {
		return "..."
	}
	runes := []rune(trimmed)
	maxRunes := maxWidth - 3
	if len(runes) <= maxRunes {
		return trimmed
	}
	return string(runes[:maxRunes]) + "..."
}

// EmbeddingModel returns the currently selected embeddings model in the UI.
func (m Model) EmbeddingModel() string {
	return m.embeddingModel
}

func newSettingsList(width int) list.Model {
	delegate := settingsDelegate{}
	l := list.New([]list.Item{}, delegate, width, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(true)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
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
	editor.KeyMap.WordForward = key.NewBinding(
		key.WithKeys("alt+right", "alt+f", "ctrl+right"),
		key.WithHelp("ctrl+right", "word forward"),
	)
	editor.KeyMap.WordBackward = key.NewBinding(
		key.WithKeys("alt+left", "alt+b", "ctrl+left"),
		key.WithHelp("ctrl+left", "word backward"),
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

func (m Model) handleSettingsEnter() (tea.Model, tea.Cmd) {
	if m.settingsMenu == nil {
		return m, nil
	}
	entry := m.selectedSettingsEntry()
	if entry == nil {
		return m, nil
	}
	if entry.Kind == SettingsEntryAction {
		switch entry.ID {
		case settingsIDModel:
			m.pickerFromSettings = true
			return m.openPicker(pickerKindModel, nil)
		case settingsIDExtractorModel:
			m.pickerFromSettings = true
			return m.openPicker(pickerKindExtractorModel, nil)
		case settingsIDEmbeddingsModel:
			m.pickerFromSettings = true
			return m.openPicker(pickerKindEmbeddingsModel, nil)
		}
		return m, nil
	}
	if entry.Kind == SettingsEntryProfile {
		return m.selectProfileEntry(entry)
	}
	if entry.Kind == SettingsEntrySubmenu {
		if next, ok := NavigateToSubmenu(m.settingsMenu, entry.ID); ok {
			m.settingsMenu = next
			m.refreshProfilesList()
			m.syncSettingsList()
			return m, nil
		}
		return m, nil
	}
	if entry.Kind == SettingsEntryValue {
		m.settingsEditing = true
		m.settingsEditEntry = entry
		m.settingsErr = ""
		m.settingsEditor = newSettingsEditor()
		// For the API key, populate with real value (not obfuscated)
		if entry.ID == settingsIDProviderAPIKey {
			m.settingsEditor.SetValue(m.providerAPIKey)
		} else {
			m.settingsEditor.SetValue(entry.Value)
		}
		return m, m.settingsEditor.Focus()
	}
	return m, nil
}

func (m Model) handleSettingsSave() (tea.Model, tea.Cmd) {
	if m.settingsEditEntry == nil {
		m.settingsEditing = false
		return m, nil
	}
	val := strings.TrimSpace(m.settingsEditor.Value())
	entry := *m.settingsEditEntry
	if m.profilesAction != nil && entry.ID == settingsIDProfiles {
		action := m.profilesAction
		if val == "" {
			m.settingsErr = "value must not be empty"
			return m, nil
		}
		p, err := action.Apply(context.Background(), m.profileService, val)
		if err != nil {
			m.settingsErr = err.Error()
			return m, nil
		}
		m.profilesAction = nil
		m.settingsEditing = false
		m.settingsEditEntry = nil
		m.settingsErr = ""

		switch action := action.(type) {
		case profilesCreateAction:
			selected, err := m.profileService.Select(context.Background(), p.Name)
			if err != nil {
				m.settingsErr = err.Error()
				return m, nil
			}
			if cmd := m.switchToProfile(selected); cmd != nil {
				return m, cmd
			}
		case profilesRenameAction:
			if strings.EqualFold(action.current, m.profileName) {
				if cmd := m.switchToProfile(p); cmd != nil {
					return m, cmd
				}
			} else {
				if cmd := m.syncActiveProfile(); cmd != nil {
					return m, cmd
				}
			}
		}
		m.refreshSettingsValues()
		m.syncSettingsList()
		return m, nil
	}
	if entry.ValueType == SettingsValueNumber {
		parsed, err := parsePositiveInt(val)
		if err != nil {
			m.settingsErr = err.Error()
			return m, nil
		}
		switch entry.ID {
		case settingsIDRawNoteDays, settingsIDWeeklySummaryWeeks:
			if m.execCtx == nil || m.execCtx.DB == nil || m.execCtx.ProfileID == "" {
				m.settingsErr = "timeline settings unavailable"
				return m, nil
			}
			repo := store.NewTimelineSettingsRepo(m.execCtx.DB)
			current, err := repo.GetOrDefault(context.Background(), m.execCtx.ProfileID)
			if err != nil {
				m.settingsErr = err.Error()
				return m, nil
			}
			switch entry.ID {
			case settingsIDRawNoteDays:
				current.RawNoteDays = parsed
				m.rawNoteDays = parsed
			case settingsIDWeeklySummaryWeeks:
				current.WeeklySummaryWeeks = parsed
				m.weeklySummaryWeeks = parsed
			}
			if err := repo.Upsert(context.Background(), current); err != nil {
				m.settingsErr = err.Error()
				return m, nil
			}
			if m.execCtx != nil && m.execCtx.DB != nil {
				_ = cache.NewInvalidationTriggers(store.NewContextCacheRepo(m.execCtx.DB)).OnTimelineSettingsChange(context.Background(), m.execCtx.ProfileID)
			}
			entry.Value = strconv.Itoa(parsed)
		default:
			entry.Value = strconv.Itoa(parsed)
		}
	}
	if entry.ValueType == SettingsValueText {
		switch entry.ID {
		case settingsIDSystemPrompt:
			m.systemPrompt = val
			entry.Value = val
			if err := m.saveSystemPrompt(val); err != nil {
				m.settingsErr = err.Error()
				return m, nil
			}
		case settingsIDProviderEndpoint:
			m.providerEndpoint = val
			entry.Value = val
			if err := m.saveProviderEndpoint(val); err != nil {
				m.settingsErr = err.Error()
				return m, nil
			}
		case settingsIDProviderAPIKey:
			m.providerAPIKey = val
			entry.Value = obfuscateKey(val)
			if err := m.saveProviderAPIKey(val); err != nil {
				m.settingsErr = err.Error()
				return m, nil
			}
		case settingsIDMonthlySummaryMonths:
			if m.execCtx == nil || m.execCtx.DB == nil || m.execCtx.ProfileID == "" {
				m.settingsErr = "timeline settings unavailable"
				return m, nil
			}
			repo := store.NewTimelineSettingsRepo(m.execCtx.DB)
			current, err := repo.GetOrDefault(context.Background(), m.execCtx.ProfileID)
			if err != nil {
				m.settingsErr = err.Error()
				return m, nil
			}
			if strings.EqualFold(val, "all_remaining") || strings.EqualFold(val, "all remaining") {
				current.MonthlySummaryMonths = store.MonthlySummaryAllRemaining
				m.monthlySummaryMonths = "all_remaining"
				entry.Value = "all_remaining"
			} else {
				parsed, err := parsePositiveInt(val)
				if err != nil {
					m.settingsErr = "value must be a positive number or all_remaining"
					return m, nil
				}
				current.MonthlySummaryMonths = parsed
				m.monthlySummaryMonths = strconv.Itoa(parsed)
				entry.Value = strconv.Itoa(parsed)
			}
			if err := repo.Upsert(context.Background(), current); err != nil {
				m.settingsErr = err.Error()
				return m, nil
			}
			if m.execCtx != nil && m.execCtx.DB != nil {
				_ = cache.NewInvalidationTriggers(store.NewContextCacheRepo(m.execCtx.DB)).OnTimelineSettingsChange(context.Background(), m.execCtx.ProfileID)
			}
		default:
			entry.Value = val
		}
	}
	m.updateSettingsEntry(entry)
	m.settingsEditing = false
	m.settingsEditEntry = nil
	m.settingsErr = ""
	m.syncSettingsList()
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
	ctx := context.Background()
	if m.execCtx.ProfileID != "" && m.execCtx.DB != nil {
		timelineRepo := store.NewTimelineSettingsRepo(m.execCtx.DB)
		if settings, err := timelineRepo.GetOrDefault(ctx, m.execCtx.ProfileID); err == nil {
			m.rawNoteDays = settings.RawNoteDays
			m.weeklySummaryWeeks = settings.WeeklySummaryWeeks
			if settings.MonthlySummaryMonths == store.MonthlySummaryAllRemaining {
				m.monthlySummaryMonths = "all_remaining"
			} else {
				m.monthlySummaryMonths = strconv.Itoa(settings.MonthlySummaryMonths)
			}
		}
	}
	if m.execCtx.ProfileID != "" && m.execCtx.DB != nil {
		promptStore := profile.NewPromptStore(m.execCtx.ProfileSlug, nil)
		if prompt, err := promptStore.GetSystemPrompt(ctx); err == nil {
			m.systemPrompt = prompt
		}
		cfgRepo := store.NewProviderConfigRepo(m.execCtx.DB)
		if cfg, err := cfgRepo.GetActive(ctx, m.execCtx.ProfileID); err == nil {
			m.providerEndpoint = cfg.Endpoint
			m.embeddingModel = cfg.EmbeddingsModel
			if pass, err := security.MachinePassphrase(); err == nil {
				if dec, err := security.Decrypt(cfg.CredentialRef, pass); err == nil {
					m.providerAPIKey = dec
				}
			}
		}
	}
	m.applySettingsValues()
	m.refreshProfilesList()
}

func (m *Model) applySettingsValues() {
	applyToMenu(m.settingsMenu, m)
}

func applyToMenu(menu *SettingsMenu, m *Model) {
	if menu == nil {
		return
	}
	entries := make([]SettingsEntry, len(menu.Entries))
	copy(entries, menu.Entries)
	for i := range entries {
		switch entries[i].ID {
		case settingsIDRawNoteDays:
			if m.rawNoteDays > 0 {
				entries[i].Value = strconv.Itoa(m.rawNoteDays)
			}
		case settingsIDWeeklySummaryWeeks:
			if m.weeklySummaryWeeks > 0 {
				entries[i].Value = strconv.Itoa(m.weeklySummaryWeeks)
			}
		case settingsIDMonthlySummaryMonths:
			entries[i].Value = m.monthlySummaryMonths
		case settingsIDSystemPrompt:
			entries[i].Value = m.systemPrompt
		case settingsIDProviderEndpoint:
			entries[i].Value = m.providerEndpoint
		case settingsIDProviderAPIKey:
			entries[i].Value = obfuscateKey(m.providerAPIKey)
		}
		if entries[i].ID == settingsIDModel {
			entries[i].Value = m.activeModel
		}
		if entries[i].ID == settingsIDExtractorModel {
			entries[i].Value = m.extractorModel
		}
		if entries[i].ID == settingsIDEmbeddingsModel {
			entries[i].Value = m.embeddingModel
		}
		if entries[i].Submenu != nil {
			applyToMenu(entries[i].Submenu, m)
		}
	}
	menu.Entries = entries
}

func obfuscateKey(key string) string {
	if len(key) == 0 {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func (m *Model) updateListHelp() {
	m.settingsList.SetShowHelp(true)
	enterSelect := key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select"))
	escBack := key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/close"))

	shortHelp := func() []key.Binding {
		if m.picker != nil {
			return []key.Binding{enterSelect, escBack}
		}
		if m.settingsOpen && m.settingsEditing {
			return []key.Binding{
				key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save")),
				key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
				key.NewBinding(key.WithKeys("alt+enter"), key.WithHelp("alt+enter", "newline")),
				key.NewBinding(key.WithKeys("ctrl+left", "ctrl+right"), key.WithHelp("ctrl+←/→", "word jump")),
			}
		}
		if m.settingsOpen && m.settingsMenu != nil && m.settingsMenu.ID == settingsIDProfiles {
			return []key.Binding{enterSelect, escBack, m.keys.profileNew, m.keys.profileRename, m.keys.profileDelete}
		}
		if m.settingsOpen {
			return []key.Binding{enterSelect, escBack}
		}
		return nil
	}
	m.settingsList.AdditionalShortHelpKeys = shortHelp
	m.settingsList.AdditionalFullHelpKeys = shortHelp
}

func (m *Model) saveProviderEndpoint(endpoint string) error {
	if m.execCtx == nil || m.execCtx.DB == nil {
		return errors.New("settings: no database available")
	}
	repo := store.NewProviderConfigRepo(m.execCtx.DB)
	return repo.SetEndpoint(context.Background(), m.execCtx.ProfileID, endpoint)
}

func (m *Model) saveProviderAPIKey(key string) error {
	if m.execCtx == nil || m.execCtx.DB == nil {
		return errors.New("settings: no database available")
	}
	pass, err := security.MachinePassphrase()
	if err != nil {
		return fmt.Errorf("settings: passphrase: %w", err)
	}
	encrypted, err := security.Encrypt(key, pass)
	if err != nil {
		return fmt.Errorf("settings: encrypt key: %w", err)
	}
	repo := store.NewProviderConfigRepo(m.execCtx.DB)
	return repo.SetCredentialRef(context.Background(), m.execCtx.ProfileID, encrypted)
}

func (m *Model) saveSystemPrompt(prompt string) error {
	if m.execCtx == nil || m.execCtx.ProfileSlug == "" {
		return errors.New("settings: no active profile available")
	}
	promptStore := profile.NewPromptStore(m.execCtx.ProfileSlug, nil)
	if err := promptStore.SetSystemPrompt(context.Background(), prompt); err != nil {
		return err
	}
	if m.execCtx.OnPromptChanged != nil {
		return m.execCtx.OnPromptChanged(m.execCtx.ProfileSlug)
	}
	return nil
}

func (m *Model) switchToProfile(p *store.Profile) tea.Cmd {
	if p == nil {
		return nil
	}
	m.profileName = p.Name
	if m.execCtx != nil {
		m.execCtx.ProfileID = p.ID
		m.execCtx.ProfileSlug = p.Slug
	}
	m.settingsMenu = DefaultSettingsMenu()
	if m.profileSwitch != nil {
		return m.profileSwitch(p.Name)
	}
	return nil
}

func (m *Model) syncActiveProfile() tea.Cmd {
	if m.profileService == nil {
		return nil
	}
	active, err := m.profileService.GetActive(context.Background())
	if err != nil {
		m.err = err
		return nil
	}
	m.err = nil
	return m.switchToProfile(active)
}

func (m *Model) handleProfileDeleteRequest() (tea.Model, tea.Cmd) {
	name := m.selectedProfileName()
	if name == "" {
		return m, nil
	}
	m.profileDeleteCandidate = name
	m.settingsErr = "press enter to confirm deleting \"" + name + "\" or esc to cancel"
	return m, nil
}

func (m *Model) confirmProfileDelete() (tea.Model, tea.Cmd) {
	if m.profileDeleteCandidate == "" {
		return m, nil
	}
	name := m.profileDeleteCandidate
	m.profileDeleteCandidate = ""
	m.settingsErr = ""
	return m.deleteSelectedProfileByName(name)
}

func (m *Model) deleteSelectedProfileByName(name string) (tea.Model, tea.Cmd) {
	if m.profileService == nil {
		return m, nil
	}
	if name == "" {
		return m, nil
	}
	confirm := func(string) bool { return true }
	if m.execCtx != nil && m.execCtx.Confirm != nil {
		confirm = m.execCtx.Confirm
	}
	if err := m.profileService.Delete(context.Background(), name, confirm); err != nil {
		if errors.Is(err, profile.ErrConfirmationRequired) {
			m.settingsErr = "delete canceled"
			m.err = nil
			return m, nil
		}
		m.err = err
		m.settingsErr = ""
		return m, nil
	}
	m.err = nil
	m.settingsErr = ""
	if cmd := m.syncActiveProfile(); cmd != nil {
		return m, cmd
	}
	m.refreshSettingsValues()
	m.syncSettingsList()
	return m, nil
}

func (m *Model) renderSettingsEditor(height int) string {
	if m.settingsEditEntry == nil {
		return ""
	}
	height = max(height-4, 6)
	m.settingsEditor.SetHeight(height)
	m.settingsEditor.SetWidth(max(m.width-8, 40))

	header := lipgloss.NewStyle().Bold(true).Render(m.settingsEditEntry.Label)
	description := settingsEditorDescription(m.settingsEditEntry.ID)
	var descBlock string
	if description != "" {
		descBlock = "\n" + helpShortStyle.Render("  "+description)
	}
	body := m.settingsEditor.View()
	var errBlock string
	if m.settingsErr != "" {
		errBlock = "\n" + errStyle.Render("  "+m.settingsErr)
	}
	return pickerBorderStyle.Render(header + descBlock + "\n" + body + errBlock)
}

func settingsEditorDescription(entryID string) string {
	switch entryID {
	case settingsIDRawNoteDays:
		return "Sets how many recent rolling days of raw notes are included. Must be greater than 0."
	case settingsIDWeeklySummaryWeeks:
		return "Sets how many weeks before the raw-note layer are represented by weekly summaries. Must be greater than 0."
	case settingsIDMonthlySummaryMonths:
		return "Sets how many older months are represented by monthly summaries. Use a positive number or all_remaining."
	case settingsIDSystemPrompt:
		return "Sets the system role prompt that guides every reply."
	default:
		return ""
	}
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
	for i := range slices.Backward(m.messages) {
		if m.messages[i].role == "pending" {
			m.messages[i].role = "assistant"
			m.messages[i].content = content
			m.messages[i].timestamp = time.Now()
			return
		}
	}
	m.messages = append(m.messages, chatMessage{role: "assistant", content: content, timestamp: time.Now()})
	m.allMessages = append([]chatMessage(nil), m.messages...)
}

func (m *Model) clearPending() {
	for i := range slices.Backward(m.messages) {
		if m.messages[i].role == "pending" {
			m.messages = append(m.messages[:i], m.messages[i+1:]...)
			m.allMessages = append([]chatMessage(nil), m.messages...)
			return
		}
	}
}

func (m *Model) updatePendingSpinner() {
	if !m.pending {
		return
	}
	for i := range slices.Backward(m.messages) {
		if m.messages[i].role == "pending" {
			m.messages[i].content = spinnerFrames[m.spinnerIndex]
			return
		}
	}
}

// SetStartupConversationHistory seeds the visible conversation history and optional non-fatal load error.
func (m *Model) SetStartupConversationHistory(messages []*store.Message, err error) {
	if err != nil {
		m.setHistoryError(err)
	}
	if len(messages) == 0 {
		return
	}
	chatMsgs := m.mapStoreMessagesToChatMessages(messages)
	m.loadInitialConversationHistory(chatMsgs)
	m.messages = m.conversationHistoryWindow.messages
	m.setHistoryError(nil)
	if m.ready {
		m.syncViewport()
	}
}

func (m *Model) scrollZoneForY(y int) scrollZone {
	if y < 0 || m.height <= 0 {
		return scrollZoneOutside
	}

	footerH := lipgloss.Height(m.renderFooter())
	inputH := lipgloss.Height(strings.TrimRight(m.input.View(), "\n"))
	if inputH <= 0 {
		inputH = 1
	}

	// Input occupies lines above the footer; extend one row upward to cover
	// divider/renderer variance across terminals.
	inputStartY := m.height - footerH - inputH - 2
	inputEndY := max(m.height-footerH-3, inputStartY)

	if y >= inputStartY && y <= inputEndY {
		return scrollZoneInput
	}
	if y >= 0 && y < m.viewport.Height() {
		return scrollZoneMessages
	}
	return scrollZoneOutside
}

func (m Model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	zone := m.scrollZoneForY(mouse.Y)
	switch zone {
	case scrollZoneOutside:
		return m, nil
	case scrollZoneMessages:
		if mouse.Button == tea.MouseWheelUp {
			var cmd tea.Cmd
			for range conversationWheelStepLines {
				m.viewport, cmd = m.viewport.Update(tea.KeyPressMsg{Code: tea.KeyUp})
			}
			m.maybeLoadOlderConversationHistory()
			return m, cmd
		}
		if mouse.Button == tea.MouseWheelDown {
			var cmd tea.Cmd
			for range conversationWheelStepLines {
				m.viewport, cmd = m.viewport.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			}
			return m, cmd
		}
	case scrollZoneInput:
		m.ensureInputHistoryLoaded()
		if mouse.Button == tea.MouseWheelUp {
			if m.navigateHistory(-1) {
				return m, nil
			}
		}
		if mouse.Button == tea.MouseWheelDown {
			if m.navigateHistory(1) {
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) toggleSidebar() (Model, tea.Cmd) {
	if m.sidebar == nil {
		return m, nil
	}
	m.sidebar.open = !m.sidebar.open
	if m.sidebar.open {
		m.sidebar.width = max(m.width/5, 36)
		mainWidth := m.width - m.sidebar.width - 1
		m.viewport.SetWidth(mainWidth)
		m.viewport.SetContent(m.renderHistory())
		m.input.SetWidth(mainWidth - 4)
		return m, m.sidebar.loadInitialBatch(context.Background())
	}
	m.viewport.SetWidth(m.width)
	m.input.SetWidth(m.width - 4)
	return m, nil
}

func (m Model) handleSidebarMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	if mouse.Button == tea.MouseWheelUp {
		var cmd tea.Cmd
		for range conversationWheelStepLines {
			m.sidebar.viewport, cmd = m.sidebar.viewport.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		}
		// Load more when scrolled to the top.
		if m.sidebar.viewport.AtTop() && m.sidebar.hasMore && !m.sidebar.loading {
			cmds := []tea.Cmd{cmd, m.sidebar.loadMore(context.Background())}
			return m, tea.Batch(cmds...)
		}
		return m, cmd
	}
	if mouse.Button == tea.MouseWheelDown {
		var cmd tea.Cmd
		for range conversationWheelStepLines {
			m.sidebar.viewport, cmd = m.sidebar.viewport.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		}
		return m, cmd
	}
	return m, nil
}

func (m *Model) syncViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.conversationHistoryWindow.messages = append([]chatMessage(nil), m.messages...)
	m.conversationHistoryWindow.loadedCnt = len(m.messages)
	if len(m.allMessages) == 0 || len(m.messages) >= len(m.allMessages) {
		m.allMessages = append([]chatMessage(nil), m.messages...)
		m.conversationHistoryWindow.hasOlder = false
	}
}

// renderHistory renders all chat messages using styled bubbles.
func (m *Model) renderHistory() string {
	if len(m.messages) == 0 {
		return "\n" + headerStyle.Render("  No messages yet — start typing below.") + "\n"
	}

	termWidth := m.viewport.Width()
	if termWidth < 40 {
		termWidth = m.width // fallback before viewport is ready
	}
	if termWidth < 40 {
		termWidth = 80
	}

	var sb strings.Builder
	for i, msg := range m.messages {
		if i > 0 {
			prev := m.messages[i-1]
			if msg.conversationID != "" && prev.conversationID != "" && msg.conversationID != prev.conversationID {
				boundaryAt := msg.conversationStartedAt
				if boundaryAt.IsZero() {
					boundaryAt = msg.timestamp
				}
				sb.WriteString(renderConversationBoundary(boundaryAt, termWidth))
				sb.WriteString("\n\n")
			}
		}
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

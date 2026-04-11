package tui

import "sort"

const (
	settingsIDModel            = "model"
	settingsIDExtractorModel   = "extractor_model"
	settingsIDSystemPrompt     = "system_prompt"
	settingsIDMemoryTokenLimit = "memory_token_budget"
	settingsIDProviders        = "providers"
	settingsIDProfiles         = "profiles"
	settingsIDThemes           = "themes"
	settingsIDProviderEndpoint = "provider_endpoint"
	settingsIDProviderAPIKey   = "provider_api_key"
)

// SettingsEntryKind describes whether an entry is a value or submenu.
type SettingsEntryKind string

const (
	SettingsEntryValue   SettingsEntryKind = "value"
	SettingsEntrySubmenu SettingsEntryKind = "submenu"
	SettingsEntryAction  SettingsEntryKind = "action"
)

// SettingsValueType indicates how a value should be edited.
type SettingsValueType string

const (
	SettingsValueText   SettingsValueType = "text"
	SettingsValueNumber SettingsValueType = "number"
	SettingsValueAction SettingsValueType = "action"
)

// SettingsEntry represents a single settings entry.
type SettingsEntry struct {
	ID        string
	Label     string
	Kind      SettingsEntryKind
	ValueType SettingsValueType
	Value     string
	Submenu   *SettingsMenu
	Source    string
}

// SettingsSource indicates the origin of a setting value.
type SettingsSource string

const (
	SettingsSourceProfile SettingsSource = "profile"
	SettingsSourceDB      SettingsSource = "db"
)

// SettingsMenu represents a settings menu level.
type SettingsMenu struct {
	ID      string
	Title   string
	Entries []SettingsEntry
	Parent  *SettingsMenu
}

// NavigateToSubmenu updates the menu to the selected submenu if present.
func NavigateToSubmenu(menu *SettingsMenu, entryID string) (*SettingsMenu, bool) {
	if menu == nil {
		return menu, false
	}
	for _, entry := range menu.Entries {
		if entry.ID == entryID && entry.Submenu != nil {
			entry.Submenu.Parent = menu
			return entry.Submenu, true
		}
	}
	return menu, false
}

// NavigateUp returns the parent menu if available.
func NavigateUp(menu *SettingsMenu) (*SettingsMenu, bool) {
	if menu == nil || menu.Parent == nil {
		return menu, false
	}
	return menu.Parent, true
}

// DefaultSettingsMenu returns the root settings menu entries.
func DefaultSettingsMenu() *SettingsMenu {
	providersMenu := &SettingsMenu{
		ID:    settingsIDProviders,
		Title: "Provider",
		Entries: []SettingsEntry{
			{
				ID:        "provider_endpoint",
				Label:     "Endpoint",
				Kind:      SettingsEntryValue,
				ValueType: SettingsValueText,
			},
			{
				ID:        "provider_api_key",
				Label:     "Key",
				Kind:      SettingsEntryValue,
				ValueType: SettingsValueText,
			},
		},
	}

	menu := &SettingsMenu{
		ID:    "settings",
		Title: "Settings",
		Entries: []SettingsEntry{
			{
				ID:        settingsIDModel,
				Label:     "Model",
				Kind:      SettingsEntryAction,
				ValueType: SettingsValueAction,
			},
			{
				ID:        settingsIDExtractorModel,
				Label:     "Model Extractor",
				Kind:      SettingsEntryAction,
				ValueType: SettingsValueAction,
			},
			{
				ID:        settingsIDSystemPrompt,
				Label:     "System Prompt",
				Kind:      SettingsEntryValue,
				ValueType: SettingsValueText,
				Source:    string(SettingsSourceDB),
			},
			{
				ID:        settingsIDMemoryTokenLimit,
				Label:     "Memory Token Budget",
				Kind:      SettingsEntryValue,
				ValueType: SettingsValueNumber,
				Source:    string(SettingsSourceProfile),
			},
			{
				ID:        settingsIDProviders,
				Label:     "Provider",
				Kind:      SettingsEntrySubmenu,
				ValueType: SettingsValueAction,
				Submenu:   providersMenu,
			},
			{
				ID:        settingsIDProfiles,
				Label:     "Profiles",
				Kind:      SettingsEntrySubmenu,
				ValueType: SettingsValueAction,
			},
			{
				ID:        settingsIDThemes,
				Label:     "Themes",
				Kind:      SettingsEntrySubmenu,
				ValueType: SettingsValueAction,
			},
		},
	}

	for i, entry := range menu.Entries {
		if entry.Submenu != nil {
			entry.Submenu.Parent = menu
			menu.Entries[i] = entry
		}
	}
	return menu
}

// SortSettingsEntries orders settings entries alphabetically by label.
func SortSettingsEntries(entries []SettingsEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Label < entries[j].Label
	})
}

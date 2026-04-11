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
)

// SettingsEntryKind describes whether an entry is a value or submenu.
type SettingsEntryKind string

const (
	SettingsEntryValue   SettingsEntryKind = "value"
	SettingsEntrySubmenu SettingsEntryKind = "submenu"
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

// SettingsMenu represents a settings menu level.
type SettingsMenu struct {
	ID      string
	Title   string
	Entries []SettingsEntry
	Parent  *SettingsMenu
}

// DefaultSettingsMenu returns the root settings menu entries.
func DefaultSettingsMenu() *SettingsMenu {
	return &SettingsMenu{
		ID:    "settings",
		Title: "Settings",
		Entries: []SettingsEntry{
			{
				ID:        settingsIDModel,
				Label:     "Model",
				Kind:      SettingsEntrySubmenu,
				ValueType: SettingsValueAction,
			},
			{
				ID:        settingsIDExtractorModel,
				Label:     "Extractor Model",
				Kind:      SettingsEntrySubmenu,
				ValueType: SettingsValueAction,
			},
			{
				ID:        settingsIDSystemPrompt,
				Label:     "System Prompt",
				Kind:      SettingsEntryValue,
				ValueType: SettingsValueText,
			},
			{
				ID:        settingsIDMemoryTokenLimit,
				Label:     "Memory Token Budget",
				Kind:      SettingsEntryValue,
				ValueType: SettingsValueNumber,
			},
			{
				ID:        settingsIDProviders,
				Label:     "Providers",
				Kind:      SettingsEntrySubmenu,
				ValueType: SettingsValueAction,
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
}

// SortSettingsEntries orders settings entries alphabetically by label.
func SortSettingsEntries(entries []SettingsEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Label < entries[j].Label
	})
}

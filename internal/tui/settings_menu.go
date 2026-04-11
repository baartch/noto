package tui

import "sort"

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

// SortSettingsEntries orders settings entries alphabetically by label.
func SortSettingsEntries(entries []SettingsEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Label < entries[j].Label
	})
}

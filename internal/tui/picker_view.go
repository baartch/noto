package tui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

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
			current := m.activeModel
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
			m.picker.setItems([]pickerItem{{Value: "", Label: "No models available"}})
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
			m.picker.setItems([]pickerItem{{Value: "", Label: "No models available"}})
		}
	case pickerKindEmbeddingsModel:
		m.picker = newPickerState("Select embeddings model", m.width-4)
		m.picker.loading = true
		if m.listEmbeddings != nil {
			current := m.embeddingModel
			cmds = append(cmds, func() tea.Msg {
				models, err := m.listEmbeddings(context.Background())
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
			m.picker.setItems([]pickerItem{{Value: "", Label: "No models available"}})
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
		if m.pickerFromSettings {
			m.pickerFromSettings = false
			return m, nil
		}
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
		if m.pickerFromSettings {
			m.pickerFromSettings = false
		} else {
			cmds = append(cmds, m.input.Focus())
		}
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
		case pickerKindEmbeddingsModel:
			if m.embeddingModelSelected != nil {
				if err := m.embeddingModelSelected(chosen); err != nil {
					m.err = err
				} else {
					m.embeddingModel = chosen
					m.embeddingModelMissing = chosen == ""
					m.messages = append(m.messages, chatMessage{role: "command", content: "Embeddings model set to: " + chosen, timestamp: time.Now()})
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

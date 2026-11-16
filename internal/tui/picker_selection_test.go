package tui

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"noto/internal/chat"
	"noto/internal/commands"
)

type selectionTestModel struct {
	model    Model
	want     string
	kind     pickerKind
	selected chan struct{}
}

func (m *selectionTestModel) Init() tea.Cmd  { return nil }
func (m *selectionTestModel) View() tea.View { return m.model.View() }
func (m *selectionTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.model.Update(msg)
	m.model = updated.(Model)
	if m.selected != nil && ((m.kind == pickerKindModel && m.model.activeModel == m.want) ||
		(m.kind == pickerKindExtractorModel && m.model.extractorModel == m.want)) {
		close(m.selected)
		m.selected = nil
	}
	return m, cmd
}

func TestModelPickerSelectionAllowsCallbacksToSendMessages(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind pickerKind
	}{{"chat model", pickerKindModel}, {"extractor model", pickerKindExtractorModel}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m := New("profile", "old-chat", "old-extractor", "", "", "", false, false,
				chat.NewDispatcher(commands.NewRegistry()), &commands.ExecContext{}, nil, nil, nil,
				nil, nil, nil, nil, nil, nil, nil, nil, false)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			m = updated.(Model)
			m.pickerKind = tc.kind
			m.picker = newPickerState("Select model", 100)
			m.picker.setItems([]pickerItem{{Value: "new-model"}})
			selected := make(chan struct{})
			wrapped := &selectionTestModel{model: m, want: "new-model", kind: tc.kind, selected: selected}
			p := tea.NewProgram(wrapped, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard))
			callback := func(string) error {
				p.Send(StatsUpdated("updated stats"))
				p.Send(ExtractorEligibilityUpdated(true))
				return nil
			}
			if tc.kind == pickerKindModel {
				wrapped.model.modelSelected = callback
			} else {
				wrapped.model.extractorModelSelected = callback
			}
			done := make(chan error, 1)
			go func() { _, err := p.Run(); done <- err }()
			go p.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
			select {
			case <-selected:
			case <-time.After(2 * time.Second):
				cancel()
				<-done
				t.Fatal("selection did not finish while callback sent messages to the program")
			}
			cancel()
			<-done
			if wrapped.model.extractorIneligible {
				t.Fatal("eligibility update was not processed")
			}
			if wrapped.model.tokenStatus != "updated stats" {
				t.Fatalf("token status = %q, want updated stats", wrapped.model.tokenStatus)
			}
		})
	}
}

func TestModelPickerSelectionFailureKeepsPreviousModel(t *testing.T) {
	m := New("profile", "old-chat", "old-extractor", "", "", "", false, false,
		chat.NewDispatcher(commands.NewRegistry()), &commands.ExecContext{}, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, false)
	m.pickerKind = pickerKindExtractorModel
	m.picker = newPickerState("Select model", 100)
	m.picker.setItems([]pickerItem{{Value: "new-model"}})
	wantErr := errors.New("unable to save model")
	m.extractorModelSelected = func(string) error { return wantErr }
	updated, cmd := m.updatePicker(tea.KeyPressMsg{Code: tea.KeyEnter}, nil)
	m = updated.(Model)
	if !m.picker.selecting || cmd == nil {
		t.Fatal("model selection did not start asynchronously")
	}
	updated, _ = m.updatePicker(tea.KeyPressMsg{Code: tea.KeyEsc}, nil)
	m = updated.(Model)
	if m.picker == nil {
		t.Fatal("picker closed while model selection was pending")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.picker != nil || m.extractorModel != "old-extractor" || !errors.Is(m.err, wantErr) {
		t.Fatalf("failed selection: picker=%v model=%q error=%v", m.picker, m.extractorModel, m.err)
	}
}

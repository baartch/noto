package integration

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTUIHelp_DynamicBindings(t *testing.T) {
	m, _ := newSettingsModel(t)

	// Toggle full help.
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	m = asModel(t, updated)

	view := m.View().Content
	if !strings.Contains(view, "ctrl+j") || !strings.Contains(view, "ctrl+l") {
		t.Fatalf("expected default help bindings in footer")
	}

	// Open settings.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	m = asModel(t, updated)
	view = m.View().Content
	if !strings.Contains(view, "enter") || !strings.Contains(view, "esc") {
		t.Fatalf("expected settings help bindings in list help")
	}
	if strings.Contains(view, "ctrl+l") {
		t.Fatalf("did not expect model picker binding in settings help")
	}

	// Navigate to profiles submenu (index 3).
	for range 3 {
		updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = asModel(t, updated)
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = asModel(t, updated)
	view = m.View().Content
	if !strings.Contains(view, "ctrl+n") || !strings.Contains(view, "ctrl+r") || !strings.Contains(view, "ctrl+d") {
		t.Fatalf("expected profiles help bindings in list help")
	}

	// Return to settings list.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = asModel(t, updated)

	// Open model picker (index 1).
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = asModel(t, updated)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = asModel(t, updated)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = asModel(t, updated)
	view = m.View().Content
	if !strings.Contains(view, "enter") || !strings.Contains(view, "esc") {
		t.Fatalf("expected picker help bindings in list help")
	}

	// Close picker and open editor (memory token budget is first entry).
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = asModel(t, updated)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = asModel(t, updated)
	view = m.View().Content
	if !strings.Contains(view, "alt+enter") || !strings.Contains(view, "ctrl+←/→") {
		t.Fatalf("expected editor help bindings in list help")
	}
}

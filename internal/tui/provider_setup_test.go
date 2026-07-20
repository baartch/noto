package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestProviderSetupState_InitialState(t *testing.T) {
	s := newProviderSetupState(80)
	if s.paginator.Page != 0 {
		t.Fatalf("expected page 0, got %d", s.paginator.Page)
	}
	if s.providerList.Index() != 0 {
		t.Fatalf("expected provider index 0, got %d", s.providerList.Index())
	}
	if s.apiInput.Value() != "" {
		t.Fatalf("expected empty API key, got %q", s.apiInput.Value())
	}
}

func TestProviderSetupState_NavigatePages(t *testing.T) {
	s := newProviderSetupState(80)

	// Right arrow moves to page 1
	updated := s.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	cmd := updated
	if cmd != nil {
		_ = cmd()
	}
	if s.paginator.Page != 1 {
		t.Fatalf("expected page 1 after right, got %d", s.paginator.Page)
	}

	// Left arrow moves back to page 0
	updated = s.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	cmd = updated
	if cmd != nil {
		_ = cmd()
	}
	if s.paginator.Page != 0 {
		t.Fatalf("expected page 0 after left, got %d", s.paginator.Page)
	}
}

func TestProviderSetupState_EscCancels(t *testing.T) {
	s := newProviderSetupState(80)
	cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for Esc")
	}
	msg := cmd()
	if _, ok := msg.(providerSetupMsg); !ok {
		t.Fatalf("expected providerSetupMsg, got %T", msg)
	}
}

func TestProviderSetupState_Page0EnterAdvances(t *testing.T) {
	s := newProviderSetupState(80)
	cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		_ = cmd()
	}
	if s.paginator.Page != 1 {
		t.Fatalf("expected page 1 after Enter on page 0, got %d", s.paginator.Page)
	}
}

func TestProviderSetupState_Page1EnterRequiresAPIKey(t *testing.T) {
	s := newProviderSetupState(80)
	// Move to page 1
	s.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	// Enter with empty API key should not produce a result
	cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected nil cmd when API key is empty")
	}
}

func TestProviderSetupState_Page1EnterConfirmsWithKey(t *testing.T) {
	s := newProviderSetupState(80)
	// Move to page 1
	s.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	// Type an API key
	s.apiInput.SetValue("sk-test-123")
	// Enter should produce a result
	cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for Enter with API key")
	}
	msg := cmd()
	psm, ok := msg.(providerSetupMsg)
	if !ok {
		t.Fatalf("expected providerSetupMsg, got %T", msg)
	}
	if psm.cancel {
		t.Fatal("expected not canceled")
	}
	if psm.result.Endpoint == "" {
		t.Fatal("expected non-empty endpoint")
	}
	if psm.result.APIKey != "sk-test-123" {
		t.Fatalf("expected API key 'sk-test-123', got %q", psm.result.APIKey)
	}
}

func TestProviderSetupState_ProviderSelectionChangesEndpoint(t *testing.T) {
	s := newProviderSetupState(80)
	// Select second provider (OpenAI)
	s.providerList.Select(1)
	s.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	s.apiInput.SetValue("sk-abc")
	cmd := s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd().(providerSetupMsg)
	if msg.result.Endpoint != "https://api.openai.com/v1" {
		t.Fatalf("expected OpenAI endpoint, got %q", msg.result.Endpoint)
	}
}

func TestProviderSetupState_ViewContainsTitle(t *testing.T) {
	s := newProviderSetupState(80)
	s.updateSize(80, 24)
	view := s.View()
	if !strings.Contains(view, "Providers") {
		t.Fatal("expected view to contain 'Providers' on page 0")
	}
}

func TestProviderSetupState_ViewPage1ContainsAPIKey(t *testing.T) {
	s := newProviderSetupState(80)
	s.updateSize(80, 24)
	s.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	view := s.View()
	if !strings.Contains(view, "API Key") {
		t.Fatal("expected view to contain 'API Key' on page 1")
	}
}

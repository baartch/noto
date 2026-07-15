package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"noto/internal/provider"
)

// providerSetupResult holds the user's selections from the setup dialog.
type providerSetupResult struct {
	Endpoint string
	APIKey   string
}

// providerSetupMsg is sent when the dialog completes.
type providerSetupMsg struct {
	result providerSetupResult
	cancel bool
}

// providerSetupState manages the two-page provider setup dialog.
type providerSetupState struct {
	paginator    paginator.Model
	providerList list.Model
	apiInput     textinput.Model
	width        int
	height       int
}

// providerListItem wraps a provider.Info for the list.Model.
type providerListItem struct {
	info     provider.Info
	selected bool
}

func (i providerListItem) Title() string       { return i.info.Name }
func (i providerListItem) Description() string { return i.info.Endpoint }
func (i providerListItem) FilterValue() string { return i.info.Name }

// providerListDelegate renders provider list items.
type providerListDelegate struct{}

func (d providerListDelegate) Height() int  { return 1 }
func (d providerListDelegate) Spacing() int { return 0 }
func (d providerListDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d providerListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(providerListItem)
	if !ok {
		return
	}
	indicator := " "
	style := pickerNormalStyle
	switch {
	case index == m.Index():
		indicator = "›"
		style = pickerCursorStyle
	case it.selected:
		indicator = "●"
		style = pickerActiveStyle
	}
	line := "  " + indicator + " " + it.info.Name
	_, _ = fmt.Fprint(w, style.Render(fitLine(line, m.Width())))
}

func newProviderSetupState(width int) *providerSetupState {
	p := paginator.New(
		paginator.WithTotalPages(2),
		paginator.WithPerPage(1),
	)
	p.Type = paginator.Dots
	p.ActiveDot = "●"
	p.InactiveDot = "○"

	items := make([]list.Item, len(provider.AvailableProviders))
	for i, prov := range provider.AvailableProviders {
		items[i] = providerListItem{info: prov}
	}
	l := list.New(items, providerListDelegate{}, width-4, 0)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.SetShowFilter(false)
	l.SetShowTitle(false)

	apiInput := textinput.New()
	apiInput.Placeholder = "sk-..."
	apiInput.EchoMode = textinput.EchoPassword
	apiInput.EchoCharacter = '•'
	apiInput.SetWidth(max(width-12, 20))
	apiInput.CharLimit = 512

	return &providerSetupState{
		paginator:    p,
		providerList: l,
		apiInput:     apiInput,
		width:        width,
	}
}

func (s *providerSetupState) updateSize(width, height int) {
	s.width = width
	s.height = height
	s.providerList.SetSize(max(width-4, 20), max(height/2-4, 4))
	s.apiInput.SetWidth(max(width-12, 20))
}

func (s *providerSetupState) selectedProvider() provider.Info {
	idx := s.providerList.Index()
	if idx < 0 || idx >= len(provider.AvailableProviders) {
		return provider.AvailableProviders[0]
	}
	return provider.AvailableProviders[idx]
}

func (s *providerSetupState) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.updateSize(msg.Width, msg.Height)
		return nil
	case tea.KeyPressMsg:
		//exhaustive:ignore
		switch {
		case msg.Key().Code == tea.KeyEsc:
			return func() tea.Msg { return providerSetupMsg{cancel: true} }

		case msg.Key().Code == tea.KeyEnter && s.paginator.Page == 1:
			apiKey := strings.TrimSpace(s.apiInput.Value())
			if apiKey == "" {
				return nil
			}
			prov := s.selectedProvider()
			result := providerSetupResult{
				Endpoint: prov.Endpoint,
				APIKey:   apiKey,
			}
			return func() tea.Msg { return providerSetupMsg{result: result} }

		case msg.Key().Code == tea.KeyEnter && s.paginator.Page == 0:
			s.paginator.NextPage()
			s.apiInput.SetValue("")
			return s.apiInput.Focus()

		case key.Matches(msg, s.paginator.KeyMap.NextPage):
			if s.paginator.Page == 0 {
				s.paginator.NextPage()
				s.apiInput.SetValue("")
				return s.apiInput.Focus()
			}
			return nil

		case key.Matches(msg, s.paginator.KeyMap.PrevPage):
			if s.paginator.Page == 1 {
				s.paginator.PrevPage()
				s.apiInput.Blur()
			}
			return nil
		}
	}

	if s.paginator.Page == 0 {
		updated, cmd := s.providerList.Update(msg)
		s.providerList = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	} else {
		updated, cmd := s.apiInput.Update(msg)
		s.apiInput = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

func (s *providerSetupState) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true).
		PaddingLeft(1)

	paginatorView := s.paginator.View()

	var content string
	switch s.paginator.Page {
	case 0:
		s.providerList.SetSize(max(s.width-6, 20), max(s.height/2-4, 4))
		listView := s.providerList.View()
		content = titleStyle.Render("Providers") + "\n" +
			"  Select a provider:\n\n" +
			listView
	case 1:
		prov := s.selectedProvider()
		content = titleStyle.Render("API Key") + "\n" +
			fmt.Sprintf("  Provider: %s (%s)\n\n", prov.Name, prov.Endpoint) +
			"  " + s.apiInput.View() + "\n\n" +
			"  Press Enter to confirm"
	}

	helpLine := "  ← → pages · Enter confirm · Esc cancel"
	borderContent := content + "\n" + helpLine + "\n" + "  " + paginatorView

	return pickerBorderStyle.Render(borderContent)
}

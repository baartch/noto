package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"noto/internal/store"
)

const (
	sidebarTabNotes   = 0
	sidebarTabWeekly  = 1
	sidebarTabMonthly = 2
	sidebarPageSize   = 20
)

type sidebarModel struct {
	open      bool
	activeTab int
	width     int
	viewport  viewport.Model

	notes   []*store.MemoryNote
	weekly  []*store.MemorySummary
	monthly []*store.MemorySummary

	hasMore bool
	loading bool

	noteRepo    *store.MemoryNoteRepo
	summaryRepo *store.MemorySummaryRepo
	profileID   string
}

func newSidebar(width int, noteRepo *store.MemoryNoteRepo, summaryRepo *store.MemorySummaryRepo, profileID string) *sidebarModel {
	return &sidebarModel{
		open:        false,
		activeTab:   sidebarTabNotes,
		width:       width,
		viewport:    viewport.New(viewport.WithWidth(max(width-2, 10)), viewport.WithHeight(10)),
		noteRepo:    noteRepo,
		summaryRepo: summaryRepo,
		profileID:   profileID,
	}
}

func (s *sidebarModel) render(height int) string {
	if !s.open {
		return ""
	}

	tabH := lipgloss.Height(s.renderTabs())
	vpHeight := max(1, height-tabH-1)
	if s.viewport.Height() != vpHeight || s.viewport.Width() != s.width-2 {
		s.viewport.SetHeight(vpHeight)
		s.viewport.SetWidth(max(s.width-2, 10))
	}

	s.viewport.SetContent(s.renderContent())
	var sb strings.Builder
	sb.WriteString(s.renderTabs())
	sb.WriteString("\n")
	sb.WriteString(s.viewport.View())

	return sidebarBorder.Render(sb.String())
}

func (s *sidebarModel) renderTabs() string {
	tabs := []string{"Notes", "Weekly", "Monthly"}
	var parts []string
	for i, label := range tabs {
		if i == s.activeTab {
			parts = append(parts, sidebarActiveTab.Render(label))
		} else {
			parts = append(parts, sidebarInactiveTab.Render(label))
		}
	}
	return strings.Join(parts, "  ")
}

func (s *sidebarModel) renderContent() string {
	switch s.activeTab {
	case sidebarTabNotes:
		return s.renderNoteList()
	case sidebarTabWeekly:
		return s.renderSummaryList(s.weekly)
	case sidebarTabMonthly:
		return s.renderSummaryList(s.monthly)
	}
	return ""
}

func (s *sidebarModel) renderNoteList() string {
	if len(s.notes) == 0 && !s.loading {
		return sidebarEmpty.Render("  No notes yet.")
	}
	var sb strings.Builder
	innerW := max(s.width-4, 10)
	for i := len(s.notes) - 1; i >= 0; i-- {
		n := s.notes[i]
		meta := fmt.Sprintf("%s  I%d  ·  %s", n.Category, n.Importance, n.CreatedAt.Format("2006-01-02 15:04"))
		body := wordWrap(n.Content, innerW)
		rendered := sidebarEntryBorder.Width(s.width - 2).Render(
			sidebarMetaStyle.Render(meta) + "\n" + body,
		)
		sb.WriteString(rendered)
		sb.WriteString("\n")
		if i > 0 {
			sb.WriteString("\n")
		}
	}
	s.appendFooter(&sb)
	return sb.String()
}

func (s *sidebarModel) renderSummaryList(list []*store.MemorySummary) string {
	if len(list) == 0 && !s.loading {
		return sidebarEmpty.Render("  No summaries yet.")
	}
	var sb strings.Builder
	innerW := max(s.width-4, 10)
	isMonthly := s.activeTab == sidebarTabMonthly
	for i := len(list) - 1; i >= 0; i-- {
		sm := list[i]
		var meta string
		if isMonthly {
			meta = sm.PeriodKey
		} else {
			meta = fmt.Sprintf("%s  ·  %s – %s", sm.PeriodKey,
				sm.PeriodStart.Format("2006-01-02"), sm.PeriodEnd.Format("2006-01-02"))
		}
		body := wordWrap(sm.Content, innerW)
		rendered := sidebarEntryBorder.Width(s.width - 2).Render(
			sidebarMetaStyle.Render(meta) + "\n" + body,
		)
		sb.WriteString(rendered)
		sb.WriteString("\n")
		if i > 0 {
			sb.WriteString("\n")
		}
	}
	s.appendFooter(&sb)
	return sb.String()
}

func (s *sidebarModel) appendFooter(sb *strings.Builder) {
	if s.loading {
		sb.WriteString(sidebarLoading.Render("  loading ..."))
	} else if s.hasMore {
		sb.WriteString(sidebarLoading.Render("  ↑ scroll older"))
	}
}

func (s *sidebarModel) loadInitialBatch(ctx context.Context) tea.Cmd {
	if s.loading {
		return nil
	}
	return s.loadPage(ctx, 0)
}

func (s *sidebarModel) loadMore(ctx context.Context) tea.Cmd {
	if s.loading || !s.hasMore {
		return nil
	}
	return s.loadPage(ctx, len(s.notes))
}

func (s *sidebarModel) loadPage(ctx context.Context, offset int) tea.Cmd {
	s.loading = true
	return func() tea.Msg {
		switch s.activeTab {
		case sidebarTabNotes:
			return s.loadNotesPage(ctx, offset)
		case sidebarTabWeekly:
			return s.loadSummariesPage(ctx, store.SummaryTypeWeekly, offset)
		case sidebarTabMonthly:
			return s.loadSummariesPage(ctx, store.SummaryTypeMonthly, offset)
		}
		return nil
	}
}

func (s *sidebarModel) loadNotesPage(ctx context.Context, offset int) tea.Msg {
	notes, hasMore, err := s.noteRepo.ListByProfilePaginated(ctx, s.profileID, sidebarPageSize, offset)
	if err != nil {
		return sidebarLoadErr{tab: sidebarTabNotes, err: err}
	}
	return sidebarBatchMsg{tab: sidebarTabNotes, notes: notes, hasMore: hasMore}
}

func (s *sidebarModel) loadSummariesPage(ctx context.Context, summaryType string, offset int) tea.Msg {
	tab := sidebarTabWeekly
	if summaryType == store.SummaryTypeMonthly {
		tab = sidebarTabMonthly
	}
	sm, hasMore, err := s.summaryRepo.ListByProfileAndTypePaginated(ctx, s.profileID, summaryType, sidebarPageSize, offset)
	if err != nil {
		return sidebarLoadErr{tab: tab, err: err}
	}
	return sidebarBatchMsg{tab: tab, summaries: sm, hasMore: hasMore}
}

// sidebarBatchMsg carries a page of loaded data.
type sidebarBatchMsg struct {
	tab       int
	notes     []*store.MemoryNote
	summaries []*store.MemorySummary
	hasMore   bool
}

type sidebarLoadErr struct {
	tab int
	err error
}

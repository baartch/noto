package tui

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

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

type noteAnimState struct {
	active    bool
	startTime time.Time
	dur       time.Duration
	stagger   time.Duration
	noteIDs   map[string]int // note ID → animation order index
}

type noteHighlightState struct {
	active    bool
	startTime time.Time
	dur       time.Duration
	noteIDs   map[string]bool
}

type sidebarAnimTick struct{}

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

	animState      noteAnimState
	highlightState noteHighlightState
	reloadOldNotes map[string]*store.MemoryNote
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

func categoryEmoji(cat store.MemoryCategory) string {
	switch cat {
	case store.CategoryFact:
		return "📝"
	case store.CategoryProgress:
		return "📈"
	case store.CategoryBlocker:
		return "🚧"
	case store.CategoryActionItem:
		return "✅"
	case store.CategoryOther:
		return "📌"
	}
	return "📌"
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
	var prevRendered bool
	for _, n := range slices.Backward(s.notes) {
		p := s.noteFadeProgress(n.ID)
		if p <= 0.0 && s.animState.active {
			continue
		}
		meta := fmt.Sprintf("%s  ★%d  ·  %s", categoryEmoji(n.Category), n.Importance, n.CreatedAt.Format("2006-01-02 15:04"))
		body := wordWrap(n.Content, innerW)

		if prevRendered {
			sb.WriteString("\n")
		}

		hp := s.highlightProgress(n.ID)
		switch {
		case hp < 1.0:
			bg := highlightBgColor(hp)
			fg := highlightFgColor(hp)
			entryStyle := lipgloss.NewStyle().
				Background(lipgloss.Color(bg)).
				Foreground(lipgloss.Color(fg)).
				Padding(0, 1).
				Width(s.width - 2)
			metaStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(fg))
			sb.WriteString(entryStyle.Render(
				metaStyle.Render(meta) + "\n" + body,
			))
		case p < 1.0:
			c := fadeColor(p)
			entryStyle := sidebarEntryBorder.Foreground(lipgloss.Color(c))
			metaStyle := sidebarMetaStyle.Foreground(lipgloss.Color(c))
			sb.WriteString(entryStyle.Width(s.width - 2).Render(
				metaStyle.Render(meta) + "\n" + body,
			))
		default:
			sb.WriteString(sidebarEntryBorder.Width(s.width - 2).Render(
				sidebarMetaStyle.Render(meta) + "\n" + body,
			))
		}
		sb.WriteString("\n")
		prevRendered = true
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
	for i, sm := range slices.Backward(list) {
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

func (s *sidebarModel) needsLoad() bool {
	switch s.activeTab {
	case sidebarTabNotes:
		return len(s.notes) == 0
	case sidebarTabWeekly:
		return len(s.weekly) == 0
	case sidebarTabMonthly:
		return len(s.monthly) == 0
	}
	return false
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

func (s *sidebarModel) reloadNotes(ctx context.Context) tea.Cmd {
	s.reloadOldNotes = make(map[string]*store.MemoryNote, len(s.notes))
	for _, n := range s.notes {
		s.reloadOldNotes[n.ID] = n
	}
	s.notes = nil
	s.hasMore = false
	s.loading = true
	return s.loadPage(ctx, 0)
}

func (s *sidebarModel) noteFadeProgress(noteID string) float64 {
	if !s.animState.active {
		return 1.0
	}
	idx, ok := s.animState.noteIDs[noteID]
	if !ok {
		return 1.0
	}
	elapsed := time.Since(s.animState.startTime)
	noteDelay := time.Duration(idx) * s.animState.stagger
	noteElapsed := elapsed - noteDelay
	if noteElapsed < 0 {
		return 0.0
	}
	p := float64(noteElapsed) / float64(s.animState.dur)
	if p > 1.0 {
		return 1.0
	}
	return p
}

func fadeColor(p float64) string {
	return strconv.Itoa(max(233, min(255, 233+int(p*19.0))))
}

func (s *sidebarModel) startAnimation(noteIDs []string) tea.Cmd {
	s.animState = noteAnimState{
		active:    true,
		startTime: time.Now(),
		dur:       700 * time.Millisecond,
		stagger:   200 * time.Millisecond,
		noteIDs:   make(map[string]int, len(noteIDs)),
	}
	for i, id := range noteIDs {
		s.animState.noteIDs[id] = i
	}
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		return sidebarAnimTick{}
	})
}

func (s *sidebarModel) startHighlight(noteIDs []string) tea.Cmd {
	s.highlightState = noteHighlightState{
		active:    true,
		startTime: time.Now(),
		dur:       5 * time.Second,
		noteIDs:   make(map[string]bool, len(noteIDs)),
	}
	for _, id := range noteIDs {
		s.highlightState.noteIDs[id] = true
	}
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		return sidebarAnimTick{}
	})
}

func (s *sidebarModel) highlightProgress(noteID string) float64 {
	if !s.highlightState.active || !s.highlightState.noteIDs[noteID] {
		return 1.0
	}
	p := float64(time.Since(s.highlightState.startTime)) / float64(s.highlightState.dur)
	if p > 1.0 {
		return 1.0
	}
	return p
}

func highlightBgColor(p float64) string {
	// transition from bright blue "33" to entry bg "233"
	return strconv.Itoa(max(0, min(255, 33+int(p*200.0))))
}

func highlightFgColor(p float64) string {
	// transition from bright white "255" to entry fg "252"
	return strconv.Itoa(max(0, min(255, 255-int(p*3.0))))
}

func (s *sidebarModel) handleAnimTick() tea.Cmd {
	animDone := !s.animState.active
	if !animDone {
		elapsed := time.Since(s.animState.startTime)
		total := len(s.animState.noteIDs)
		if total == 0 {
			s.animState.active = false
			animDone = true
		} else {
			lastDone := elapsed >= s.animState.dur+time.Duration(total-1)*s.animState.stagger
			if lastDone {
				s.animState.active = false
				s.animState.noteIDs = nil
				animDone = true
			}
		}
	}

	highlightDone := !s.highlightState.active
	if !highlightDone {
		if time.Since(s.highlightState.startTime) >= s.highlightState.dur {
			s.highlightState.active = false
			s.highlightState.noteIDs = nil
			highlightDone = true
		}
	}

	if animDone && highlightDone {
		return nil
	}
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		return sidebarAnimTick{}
	})
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

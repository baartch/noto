package tui

// footerStatusViewModel describes always-visible footer fields.
type footerStatusViewModel struct {
	TokenStatus string
	CtxStatus   string
	ProfileName string
	MainModel   string
	Version     string
	HelpKey     string
}

func (m Model) footerStatus() footerStatusViewModel {
	helpKey := "Ctrl+H"
	return footerStatusViewModel{
		TokenStatus: m.tokenStatus,
		CtxStatus:   m.cacheStatus,
		ProfileName: m.profileName,
		MainModel:   m.activeModel,
		Version:     "",
		HelpKey:     helpKey,
	}
}

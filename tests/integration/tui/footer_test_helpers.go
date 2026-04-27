package tui

import "strings"

func footerHasAllFields(footer string) bool {
	checks := []string{"↑", "↓", "cr:", "cw:", "$", "ctx:", "Ctrl+H"}
	for _, c := range checks {
		if !strings.Contains(footer, c) {
			return false
		}
	}
	return true
}

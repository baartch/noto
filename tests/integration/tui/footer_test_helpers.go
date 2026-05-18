package tui

import "strings"

func footerHasAllFields(footer string) bool {
	checks := []string{"↑", "↓", "cr:", "cw:", "$", "ctx:", "Profile", "main-model"}
	for _, c := range checks {
		if !strings.Contains(footer, c) {
			return false
		}
	}
	return true
}

func containsAny(s string, opts ...string) bool {
	for _, o := range opts {
		if strings.Contains(s, o) {
			return true
		}
	}
	return false
}

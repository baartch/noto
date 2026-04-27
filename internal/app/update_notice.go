package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"noto/internal/tui"
	"noto/internal/update"
	"noto/internal/version"
)

func startUpdateCheckAsync(send func(tea.Msg)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		checker := update.NewChecker()
		res, err := checker.Check(ctx, version.String())
		if err != nil {
			return
		}
		if res.HasUpdate && send != nil {
			send(tui.UpdateAvailableNotice("Update available: " + res.Latest))
		}
	}()
}

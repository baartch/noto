package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"noto/internal/update"
	"noto/internal/version"
)

func startUpdateCheckAsync(out io.Writer) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		checker := update.NewChecker()
		res, err := checker.Check(ctx, version.String())
		if err != nil {
			return
		}
		if res.HasUpdate {
			fmt.Fprintf(out, "Update available: %s (current %s)\n", res.Latest, res.Current)
		}
	}()
}

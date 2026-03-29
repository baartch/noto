package integration

import (
	"context"
	"fmt"
	"testing"

	"noto/internal/profile"
	"noto/internal/store"
)

func BenchmarkProfileDiscovery_List100(b *testing.B) {
	db, closeDB := tempDBForBenchmark(b)
	defer closeDB()

	ctx := context.Background()
	svc := profile.NewService(store.NewProfileRepo(db))

	for i := range 100 {
		if _, err := svc.Create(ctx, fmt.Sprintf("Profile %d", i)); err != nil {
			b.Fatalf("create profile: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		profiles, warnings, err := profile.DiscoverProfiles()
		if err != nil {
			b.Fatalf("discover: %v", err)
		}
		if len(warnings) != 0 {
			b.Fatalf("unexpected warnings: %v", warnings)
		}
		if len(profiles) != 100 {
			b.Fatalf("expected 100 profiles, got %d", len(profiles))
		}
	}
}

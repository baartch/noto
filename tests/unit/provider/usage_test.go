package provider_test

import (
	"testing"

	"noto/internal/provider"
)

func TestValidateUsage_AcceptsValid(t *testing.T) {
	u := validUsageFixture()
	if err := provider.ValidateUsage(u); err != nil {
		t.Fatalf("expected valid usage, got error: %v", err)
	}
}

func TestValidateUsage_RejectsNegative(t *testing.T) {
	u := validUsageFixture()
	u.CachedTokens = -1
	if err := provider.ValidateUsage(u); err == nil {
		t.Fatalf("expected validation error")
	}
}

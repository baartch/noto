package commands

import (
	"context"
	"fmt"

	"noto/internal/store"
)

// NoopProfileService is a ProfileService that returns errors for all operations.
// Useful in tests that only care about command registration/parsing, not execution.
type NoopProfileService struct{}

// Create returns a noop error for ProfileService tests.
func (NoopProfileService) Create(_ context.Context, _ string) (*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}

// List returns a noop error for ProfileService tests.
func (NoopProfileService) List(_ context.Context) ([]*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}

// Select returns a noop error for ProfileService tests.
func (NoopProfileService) Select(_ context.Context, _ string) (*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}

// Rename returns a noop error for ProfileService tests.
func (NoopProfileService) Rename(_ context.Context, _, _ string) (*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}

// Delete returns a noop error for ProfileService tests.
func (NoopProfileService) Delete(_ context.Context, _ string, _ func(string) bool) error {
	return fmt.Errorf("noop")
}

// GetActive returns a noop error for ProfileService tests.
func (NoopProfileService) GetActive(_ context.Context) (*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}

package commands

import (
	"context"
	"fmt"

	"noto/internal/store"
)

// NoopProfileService is a ProfileService that returns errors for all operations.
// Useful in tests that only care about command registration/parsing, not execution.
type NoopProfileService struct{}

func (NoopProfileService) Create(_ context.Context, _ string) (*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}
func (NoopProfileService) List(_ context.Context) ([]*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}
func (NoopProfileService) Select(_ context.Context, _ string) (*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}
func (NoopProfileService) Rename(_ context.Context, _, _ string) (*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}
func (NoopProfileService) Delete(_ context.Context, _ string, _ func(string) bool) error {
	return fmt.Errorf("noop")
}
func (NoopProfileService) GetActive(_ context.Context) (*store.Profile, error) {
	return nil, fmt.Errorf("noop")
}

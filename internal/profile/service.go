package profile

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"noto/internal/config"
	"noto/internal/store"
)

// ErrProfileInUse is returned when attempting to delete the only remaining profile.
var ErrProfileInUse = errors.New("profile: cannot delete the only remaining profile")

// ErrConfirmationRequired is returned when a destructive action was not confirmed.
var ErrConfirmationRequired = errors.New("profile: explicit confirmation required")

// slugRe is the regex for a valid profile slug.
var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// Service manages the profile lifecycle.
type Service struct {
	repo *store.ProfileRepo
}

// NewService creates a new profile Service.
func NewService(repo *store.ProfileRepo) *Service {
	return &Service{repo: repo}
}

// Create creates a new profile with the given name.
func (s *Service) Create(ctx context.Context, name string) (*store.Profile, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("profile: name must not be empty")
	}
	slug := toSlug(name)
	id := newID()

	systemPromptPath, err := config.ProfileSystemPromptPath(slug)
	if err != nil {
		return nil, err
	}
	dbPath, err := config.ProfileDBPath(slug)
	if err != nil {
		return nil, err
	}

	// Ensure directories exist.
	if err := config.EnsureAppDirs(slug); err != nil {
		return nil, err
	}

	p := &store.Profile{
		ID:               id,
		Name:             name,
		Slug:             slug,
		SystemPromptPath: systemPromptPath,
		DBPath:           dbPath,
		IsDefault:        false,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		if errors.Is(err, store.ErrProfileNameConflict) {
			return nil, fmt.Errorf("profile: a profile named %q already exists", name)
		}
		return nil, err
	}
	return p, nil
}

// List returns all profiles.
func (s *Service) List(ctx context.Context) ([]*store.Profile, error) {
	return s.repo.List(ctx)
}

// Select sets the given profile as default (active).
func (s *Service) Select(ctx context.Context, name string) (*store.Profile, error) {
	p, err := s.repo.GetByName(ctx, name)
	if err != nil {
		if errors.Is(err, store.ErrProfileNotFound) {
			return nil, fmt.Errorf("profile: no profile named %q", name)
		}
		return nil, err
	}
	if err := s.repo.SetDefault(ctx, p.ID); err != nil {
		return nil, err
	}
	if err := s.repo.Touch(ctx, p.ID); err != nil {
		return nil, err
	}
	p.IsDefault = true
	return p, nil
}

// Rename renames a profile, updating slug and filesystem paths.
func (s *Service) Rename(ctx context.Context, oldName, newName string) (*store.Profile, error) {
	if strings.TrimSpace(newName) == "" {
		return nil, fmt.Errorf("profile: new name must not be empty")
	}
	p, err := s.repo.GetByName(ctx, oldName)
	if err != nil {
		if errors.Is(err, store.ErrProfileNotFound) {
			return nil, fmt.Errorf("profile: no profile named %q", oldName)
		}
		return nil, err
	}

	newSlug := toSlug(newName)
	newSystemPromptPath, _ := config.ProfileSystemPromptPath(newSlug)
	newDBPath, _ := config.ProfileDBPath(newSlug)

	p.Name = newName
	p.Slug = newSlug
	p.SystemPromptPath = newSystemPromptPath
	p.DBPath = newDBPath

	if err := s.repo.Update(ctx, p); err != nil {
		if errors.Is(err, store.ErrProfileNameConflict) {
			return nil, fmt.Errorf("profile: a profile named %q already exists", newName)
		}
		return nil, err
	}
	return p, nil
}

// Delete removes a profile after verifying there is more than one profile and the
// confirm function returns true.
func (s *Service) Delete(ctx context.Context, name string, confirm func(string) bool) error {
	count, err := s.repo.Count(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrProfileInUse
	}

	p, err := s.repo.GetByName(ctx, name)
	if err != nil {
		if errors.Is(err, store.ErrProfileNotFound) {
			return fmt.Errorf("profile: no profile named %q", name)
		}
		return err
	}

	msg := fmt.Sprintf("Are you sure you want to permanently delete profile %q and all its data? [yes/no]", name)
	if confirm != nil && !confirm(msg) {
		return ErrConfirmationRequired
	}

	// If deleted profile was default, pick another.
	if p.IsDefault {
		if err := s.reassignDefault(ctx, p.ID); err != nil {
			return err
		}
	}

	return s.repo.Delete(ctx, p.ID)
}

// GetActive returns the currently active (default) profile.
func (s *Service) GetActive(ctx context.Context) (*store.Profile, error) {
	p, err := s.repo.GetDefault(ctx)
	if err != nil {
		if errors.Is(err, store.ErrProfileNotFound) {
			return nil, fmt.Errorf("profile: no active profile set")
		}
		return nil, err
	}
	return p, nil
}

// LastUsed returns the most recently updated profile.
func (s *Service) LastUsed(ctx context.Context) (*store.Profile, error) {
	p, err := s.repo.GetLastUsed(ctx)
	if err != nil {
		if errors.Is(err, store.ErrProfileNotFound) {
			return nil, fmt.Errorf("profile: no profiles found")
		}
		return nil, err
	}
	return p, nil
}

// ---- helpers ----------------------------------------------------------------

func (s *Service) reassignDefault(ctx context.Context, excludeID string) error {
	all, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	for _, p := range all {
		if p.ID != excludeID {
			return s.repo.SetDefault(ctx, p.ID)
		}
	}
	return nil
}

// toSlug converts a display name to a URL-safe slug.
func toSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = fmt.Sprintf("profile-%d", time.Now().UnixNano())
	}
	return s
}

// newID generates a simple UUID-like ID using the current timestamp and random bytes.
func newID() string {
	// Use a simple time-based approach for now; replace with crypto/rand UUID in production.
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

package store

import (
	"context"
	"errors"
	"time"
)

// ErrProfileNotFound is returned when a profile lookup finds no matching record.
var ErrProfileNotFound = errors.New("store: profile not found")

// ErrProfileNameConflict is returned when a profile name or slug already exists.
var ErrProfileNameConflict = errors.New("store: profile name already exists")

// Profile is the data model for a Noto profile.
type Profile struct {
	ID               string
	Name             string
	Slug             string
	SystemPromptPath string
	DBPath           string
	IsDefault        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ProfileRepo manages CRUD operations for profiles.
type ProfileRepo struct {
	db *DB
}

// NewProfileRepo creates a new ProfileRepo.
func NewProfileRepo(db *DB) *ProfileRepo {
	return &ProfileRepo{db: db}
}

// Create is a no-op; profile metadata is not stored in the global DB.
func (r *ProfileRepo) Create(ctx context.Context, p *Profile) error {
	_ = ctx
	_ = p
	return nil
}

// GetByID is not supported; profile metadata is stored on disk.
func (r *ProfileRepo) GetByID(ctx context.Context, id string) (*Profile, error) {
	_ = ctx
	_ = id
	return nil, ErrProfileNotFound
}

// GetBySlug is not supported; profile metadata is stored on disk.
func (r *ProfileRepo) GetBySlug(ctx context.Context, slug string) (*Profile, error) {
	_ = ctx
	_ = slug
	return nil, ErrProfileNotFound
}

// GetByName is not supported; profile metadata is stored on disk.
func (r *ProfileRepo) GetByName(ctx context.Context, name string) (*Profile, error) {
	_ = ctx
	_ = name
	return nil, ErrProfileNotFound
}

// GetDefault is not supported; use instance-local active profile config instead.
func (r *ProfileRepo) GetDefault(ctx context.Context) (*Profile, error) {
	_ = ctx
	return nil, ErrProfileNotFound
}

// GetLastUsed is not supported; profile metadata is stored on disk.
func (r *ProfileRepo) GetLastUsed(ctx context.Context) (*Profile, error) {
	_ = ctx
	return nil, ErrProfileNotFound
}

// List is not supported; use filesystem discovery instead.
func (r *ProfileRepo) List(ctx context.Context) ([]*Profile, error) {
	_ = ctx
	return nil, ErrProfileNotFound
}

// Update is a no-op; profile metadata is not stored in the global DB.
func (r *ProfileRepo) Update(ctx context.Context, p *Profile) error {
	_ = ctx
	_ = p
	return nil
}

// SetDefault is a no-op; active profile is stored locally.
func (r *ProfileRepo) SetDefault(ctx context.Context, id string) error {
	_ = ctx
	_ = id
	return nil
}

// Touch is a no-op; profile metadata is stored on disk.
func (r *ProfileRepo) Touch(ctx context.Context, id string) error {
	_ = ctx
	_ = id
	return nil
}

// Delete is a no-op; profile metadata is stored on disk.
func (r *ProfileRepo) Delete(ctx context.Context, id string) error {
	_ = ctx
	_ = id
	return nil
}

// Count returns 0 because the global DB holds no profiles.
func (r *ProfileRepo) Count(ctx context.Context) (int, error) {
	_ = ctx
	return 0, nil
}

package reference

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"noto/internal/provider"
	"noto/internal/store"
)

type Diagnostic struct {
	FileID, RelativePath, Code, Message string
}

type Model struct {
	Adapter    provider.Adapter
	ContextMax int
}

type Options struct {
	Root, ProfileID string
	Repo            *store.ReferenceFileRepo
	Reader          func(context.Context, *os.Root, string) (Document, error)
	Model           func() Model
	OnUsage         func(provider.Usage)
}

type Service struct {
	opts   Options
	root   *os.Root
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	scanMu sync.Mutex
	closed bool
}

func NewService(opts Options) (*Service, error) {
	if opts.Root == "" || opts.ProfileID == "" || opts.Repo == nil {
		return nil, errors.New("reference: root, profile, and catalog are required")
	}
	info, err := os.Lstat(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("reference: inspect files directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("reference: files root must be a real directory")
	}
	root, err := os.OpenRoot(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("reference: open files directory: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if opts.Reader == nil {
		opts.Reader = Read
	}
	return &Service{opts: opts, root: root, ctx: ctx, cancel: cancel}, nil
}

// Close cancels in-flight work and releases the profile-rooted filesystem handle.
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.cancel()
	s.mu.Unlock()

	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	return s.root.Close()
}

func (s *Service) operationContext(parent context.Context) (context.Context, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, nil, errors.New("reference: service is closed")
	}
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(s.ctx, cancel)
	return ctx, func() { stop(); cancel() }, nil
}

func (s *Service) supportedModel() Model {
	if s.opts.Model == nil {
		return Model{}
	}
	return s.opts.Model()
}

func supportedPath(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf", ".txt", ".md":
		return true
	default:
		return false
	}
}

func newFileID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("reference: create file id: %w", err)
	}
	return "rf-" + hex.EncodeToString(random[:]), nil
}

func relativeName(name string) string { return filepath.ToSlash(name) }

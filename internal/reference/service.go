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

// Diagnostic describes a bounded, user-visible reference indexing or selection issue.
type Diagnostic struct {
	FileID, RelativePath, Code, Message string
}

// Model carries the configured extractor adapter and its context limit.
type Model struct {
	Adapter    provider.Adapter
	ContextMax int
}

// Options configures one profile-local reference-file service.
type Options struct {
	Root, ProfileID string
	Repo            *store.ReferenceFileRepo
	Reader          func(context.Context, *os.Root, string) (Document, error)
	Model           func() Model
	OnUsage         func(provider.Usage)
}

// Attachment is a temporary source snapshot selected for one chat turn.
type Attachment struct {
	ID, RelativePath, Fingerprint, Text string
}

// Input contains the current turn and bounded context used for file selection.
type Input struct {
	UserMessage        string
	RecentUserMessages []string
	BaseMessages       []provider.Message
	ContextMax         int
}

// Turn contains the stable catalog and temporary references prepared for a chat request.
type Turn struct {
	Catalog     string
	Attachments []Attachment
	Diagnostics []Diagnostic
	Degraded    bool
}

// Service indexes a profile's reference files and holds temporary topic context.
type Service struct {
	opts   Options
	root   *os.Root
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	scanMu sync.Mutex
	closed bool
	topic  string
	active map[string]Attachment
}

// NewService opens a rooted profile files directory and initializes empty session context.
func NewService(opts Options) (*Service, error) {
	if opts.Root == "" || opts.ProfileID == "" || opts.Repo == nil {
		return nil, errors.New("reference: root, profile, and catalog are required")
	}
	root, err := openValidatedRoot(opts.Root, os.Lstat, os.OpenRoot)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	if opts.Reader == nil {
		opts.Reader = Read
	}
	return &Service{opts: opts, root: root, ctx: ctx, cancel: cancel, active: make(map[string]Attachment)}, nil
}

func openValidatedRoot(path string, inspect func(string) (os.FileInfo, error), open func(string) (*os.Root, error)) (*os.Root, error) {
	info, err := inspect(path)
	if err != nil {
		return nil, fmt.Errorf("reference: inspect files directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("reference: files root must be a real directory")
	}
	root, err := open(path)
	if err != nil {
		return nil, fmt.Errorf("reference: open files directory: %w", err)
	}
	opened, err := root.Stat(".")
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("reference: verify opened files directory: %w", err)
	}
	if !opened.IsDir() || !os.SameFile(info, opened) {
		_ = root.Close()
		return nil, errors.New("reference: files directory changed while opening")
	}
	return root, nil
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

package reference

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"noto/internal/store"
)

type scannedFile struct {
	path string
	info os.FileInfo
}

// Scan indexes new and changed supported files and reconciles confirmed removals.
func (s *Service) Scan(parent context.Context) ([]Diagnostic, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	ctx, done, err := s.operationContext(parent)
	if err != nil {
		return nil, err
	}
	defer done()

	files, err := s.enumerate(ctx)
	if err != nil {
		return nil, err
	}
	existing, err := s.opts.Repo.List(ctx, s.opts.ProfileID)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]store.ReferenceFile, len(existing))
	for _, file := range existing {
		byPath[file.RelativePath] = file
	}

	seen := make([]string, 0, len(files))
	var diagnostics []Diagnostic
	for _, entry := range files {
		if err := ctx.Err(); err != nil {
			return diagnostics, err
		}
		seen = append(seen, entry.path)
		old, exists := byPath[entry.path]
		doc, readErr := s.opts.Reader(ctx, s.root, entry.path)
		if readErr != nil {
			row, idErr := failedRow(old, exists, s.opts.ProfileID, entry.path, entry.info, readErr)
			if idErr != nil {
				return diagnostics, idErr
			}
			if err := s.opts.Repo.Upsert(ctx, row); err != nil {
				return diagnostics, err
			}
			diagnostics = append(diagnostics, diagnostic(row, "read_failed", readErr))
			continue
		}
		if exists && old.Status == store.ReferenceFileReady && old.Fingerprint == doc.Fingerprint && old.SummaryFingerprint == doc.Fingerprint {
			if old.Size != doc.Size || old.ModTimeNS != doc.ModTimeNS || old.MediaType != doc.MediaType {
				old.Size, old.ModTimeNS, old.MediaType = doc.Size, doc.ModTimeNS, doc.MediaType
				if err := s.opts.Repo.Upsert(ctx, old); err != nil {
					return diagnostics, err
				}
			}
			continue
		}

		row := old
		if !exists {
			row.ID, err = newFileID()
			if err != nil {
				return diagnostics, err
			}
		}
		row.ProfileID, row.RelativePath = s.opts.ProfileID, entry.path
		row.MediaType, row.Fingerprint = doc.MediaType, doc.Fingerprint
		row.Size, row.ModTimeNS = doc.Size, doc.ModTimeNS
		row.Summary, row.SummaryFingerprint, row.LastError = "", "", ""
		row.Status, row.IndexedAt = store.ReferenceFilePending, nil
		if err := s.opts.Repo.Upsert(ctx, row); err != nil {
			return diagnostics, err
		}

		summary, summarizeErr := s.summarizeDocument(ctx, entry.path, doc)
		if summarizeErr != nil {
			row.Status = store.ReferenceFileFailed
			row.LastError = boundedError(summarizeErr)
			if err := s.opts.Repo.Upsert(ctx, row); err != nil {
				return diagnostics, err
			}
			diagnostics = append(diagnostics, diagnostic(row, "summary_failed", summarizeErr))
			continue
		}
		current, checkErr := s.opts.Reader(ctx, s.root, entry.path)
		if checkErr != nil || current.Fingerprint != doc.Fingerprint {
			changedErr := ErrChanged
			if checkErr != nil {
				changedErr = checkErr
			}
			row.Status = store.ReferenceFileFailed
			row.LastError = boundedError(changedErr)
			if err := s.opts.Repo.Upsert(ctx, row); err != nil {
				return diagnostics, err
			}
			diagnostics = append(diagnostics, diagnostic(row, "changed_during_index", changedErr))
			continue
		}
		row.Status = store.ReferenceFileReady
		row.Summary, row.SummaryFingerprint = summary, doc.Fingerprint
		row.LastError = ""
		indexedAt := time.Now().UTC()
		row.IndexedAt = &indexedAt
		if err := s.opts.Repo.Upsert(ctx, row); err != nil {
			return diagnostics, err
		}
	}

	if _, err := s.opts.Repo.DeleteMissing(ctx, s.opts.ProfileID, seen); err != nil {
		return diagnostics, err
	}
	return diagnostics, nil
}

func (s *Service) enumerate(ctx context.Context) ([]scannedFile, error) {
	var files []scannedFile
	var walk func(string) error
	walk = func(dir string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := dir
		if name == "" {
			name = "."
		}
		f, err := s.root.Open(name)
		if err != nil {
			return fmt.Errorf("reference: enumerate directory: %w", err)
		}
		defer func() { _ = f.Close() }()
		for {
			entries, readErr := f.ReadDir(128)
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					return err
				}
				child := entry.Name()
				if dir != "" {
					child = path.Join(dir, child)
				}
				info, err := s.root.Lstat(child)
				if err != nil {
					return fmt.Errorf("reference: inspect directory entry: %w", err)
				}
				if info.Mode()&os.ModeSymlink != 0 {
					continue
				}
				if info.IsDir() {
					if err := walk(child); err != nil {
						return err
					}
					continue
				}
				if info.Mode().IsRegular() && supportedPath(child) {
					files = append(files, scannedFile{path: relativeName(child), info: info})
				}
			}
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return fmt.Errorf("reference: enumerate directory: %w", readErr)
			}
			if errors.Is(readErr, io.EOF) || len(entries) == 0 {
				return nil
			}
		}
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	return files, nil
}

func failedRow(old store.ReferenceFile, exists bool, profileID, name string, info os.FileInfo, cause error) (store.ReferenceFile, error) {
	if !exists {
		id, err := newFileID()
		if err != nil {
			return store.ReferenceFile{}, err
		}
		old.ID = id
	}
	old.ProfileID, old.RelativePath = profileID, name
	old.MediaType = mediaTypeFor(name)
	old.Summary, old.SummaryFingerprint = "", ""
	old.Status, old.LastError = store.ReferenceFileFailed, boundedError(cause)
	old.Size, old.ModTimeNS = info.Size(), info.ModTime().UnixNano()
	old.IndexedAt = nil
	return old, nil
}

func mediaTypeFor(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf":
		return "application/pdf"
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	default:
		return ""
	}
}

func boundedError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrUnsupported):
		return ErrUnsupported.Error()
	case errors.Is(err, ErrUnsafePath):
		return ErrUnsafePath.Error()
	case errors.Is(err, ErrInvalidText):
		return ErrInvalidText.Error()
	case errors.Is(err, ErrNoText):
		return ErrNoText.Error()
	case errors.Is(err, ErrTooLarge):
		return ErrTooLarge.Error()
	case errors.Is(err, ErrChanged):
		return ErrChanged.Error()
	case errors.Is(err, context.Canceled):
		return "reference: operation canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "reference: operation timed out"
	case errors.Is(err, ErrUnreadable):
		return ErrUnreadable.Error()
	default:
		return "reference: operation failed"
	}
}

func diagnostic(file store.ReferenceFile, code string, err error) Diagnostic {
	return Diagnostic{FileID: file.ID, RelativePath: file.RelativePath, Code: code, Message: boundedError(err)}
}

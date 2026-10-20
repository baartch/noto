package reference

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Read reads one supported document from a profile-rooted directory. Source
// bytes are bounded, hashed, and then parsed from the same in-memory snapshot.
func Read(ctx context.Context, root *os.Root, relativePath string) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	if root == nil || !filepath.IsLocal(filepath.FromSlash(relativePath)) || strings.Contains(relativePath, "\\") || path.Clean(relativePath) != relativePath {
		return Document{}, ErrUnsafePath
	}
	ext := strings.ToLower(path.Ext(relativePath))
	mediaType := ""
	switch ext {
	case ".txt":
		mediaType = "text/plain"
	case ".md":
		mediaType = "text/markdown"
	case ".pdf":
		mediaType = "application/pdf"
	default:
		return Document{}, ErrUnsupported
	}
	if strings.TrimSpace(relativePath) == "" || strings.HasPrefix(relativePath, "/") {
		return Document{}, ErrUnsafePath
	}

	parts := strings.Split(relativePath, "/")
	for i := range parts {
		if parts[i] == "" || parts[i] == "." || parts[i] == ".." {
			return Document{}, ErrUnsafePath
		}
		component := path.Join(parts[:i+1]...)
		info, err := root.Lstat(component)
		if err != nil {
			return Document{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return Document{}, ErrUnsafePath
		}
		if i < len(parts)-1 && !info.IsDir() {
			return Document{}, ErrUnsafePath
		}
	}

	before, err := root.Lstat(relativePath)
	if err != nil {
		return Document{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if !before.Mode().IsRegular() {
		return Document{}, ErrUnsafePath
	}
	if before.Size() < 0 || before.Size() > MaxFileBytes {
		return Document{}, ErrTooLarge
	}
	f, err := root.Open(relativePath)
	if err != nil {
		return Document{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return Document{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return Document{}, ErrChanged
	}
	if opened.Size() < 0 || opened.Size() > MaxFileBytes {
		return Document{}, ErrTooLarge
	}
	data, err := readBounded(ctx, f, MaxFileBytes)
	if err != nil {
		return Document{}, err
	}
	after, err := f.Stat()
	if err != nil {
		return Document{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return Document{}, ErrChanged
	}
	stillAtPath, err := root.Lstat(relativePath)
	if err != nil || !os.SameFile(after, stillAtPath) || stillAtPath.Mode()&os.ModeSymlink != 0 {
		return Document{}, ErrChanged
	}

	var text string
	if ext == ".pdf" {
		text, err = extractPDF(ctx, data)
		if err != nil {
			return Document{}, err
		}
	} else {
		if !utf8.Valid(data) {
			return Document{}, ErrInvalidText
		}
		text = string(data)
	}
	if len(text) > MaxExtractedText {
		return Document{}, ErrTooLarge
	}
	if strings.TrimSpace(text) == "" {
		return Document{}, ErrNoText
	}

	return Document{
		Text: text, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(data)),
		MediaType: mediaType, Size: int64(len(data)), ModTimeNS: after.ModTime().UnixNano(),
	}, nil
}

func readBounded(ctx context.Context, r io.Reader, max int64) ([]byte, error) {
	var out bytes.Buffer
	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := r.Read(buf)
		if n > 0 {
			if int64(out.Len()+n) > max {
				return nil, ErrTooLarge
			}
			_, _ = out.Write(buf[:n])
		}
		if errors.Is(err, io.EOF) {
			return out.Bytes(), nil
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
		}
	}
}

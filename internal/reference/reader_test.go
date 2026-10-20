package reference

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadReference(t *testing.T) {
	rootDir := t.TempDir()
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Mkdir(filepath.Join(rootDir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	inputs := map[string][]byte{
		"voice.TXT":          []byte("bonjour, 世界\n"),
		"nested/identity.md": []byte("Artist voice\n"),
		"identity.pdf":       minimalPDF("Artist voice", true),
		"scan.pdf":           minimalPDF("", false),
		"empty.txt":          nil,
		"invalid.txt":        {0xff, 0xfe},
		"broken.pdf":         []byte("%PDF-invalid"),
	}
	for name, data := range inputs {
		if err := os.WriteFile(filepath.Join(rootDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name    string
		want    string
		wantErr error
	}{
		{"plain text", "voice.TXT", nil},
		{"nested markdown", "nested/identity.md", nil},
		{"extractable PDF", "identity.pdf", nil},
		{"scanned PDF", "scan.pdf", ErrNoText},
		{"invalid UTF-8", "invalid.txt", ErrInvalidText},
		{"empty text", "empty.txt", ErrNoText},
		{"malformed PDF", "broken.pdf", ErrUnreadable},
		{"unsupported", "image.png", ErrUnsupported},
		{"traversal", "../outside.txt", ErrUnsafePath},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Read(context.Background(), root, tc.want)
			if tc.wantErr != nil {
				if !errorsIs(err, tc.wantErr) {
					t.Fatalf("Read() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "plain text" && doc.Text != string(inputs[tc.want]) {
				t.Fatalf("text = %q", doc.Text)
			}
			if strings.Contains(tc.name, "PDF") && !strings.Contains(doc.Text, "Artist voice") {
				t.Fatalf("PDF text = %q", doc.Text)
			}
			wantHash := fmt.Sprintf("%x", sha256.Sum256(inputs[tc.want]))
			if doc.Fingerprint != wantHash {
				t.Fatalf("fingerprint = %s, want SHA-256 %s", doc.Fingerprint, wantHash)
			}
		})
	}
}

func TestReferenceRootSecurityAndLimits(t *testing.T) {
	rootDir := t.TempDir()
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(rootDir, "link.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := os.Mkdir(filepath.Join(rootDir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(rootDir, "nested", "link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "too-large.txt"), bytes.Repeat([]byte("x"), MaxFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "too-large-text.txt"), bytes.Repeat([]byte("x"), MaxExtractedText+1), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"link.txt", "nested/link.txt", "too-large.txt", "too-large-text.txt", "../outside.txt", "a\\..\\outside.txt"} {
		_, err := Read(context.Background(), root, path)
		if path == "too-large-text.txt" {
			if err != ErrTooLarge {
				t.Errorf("Read(%q) error = %v, want ErrTooLarge", path, err)
			}
		} else if err == nil {
			t.Errorf("Read(%q) unexpectedly succeeded", path)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Read(ctx, root, "too-large.txt"); err == nil || err != context.Canceled {
		t.Fatalf("canceled read error = %v", err)
	}
}

func minimalPDF(text string, includeText bool) []byte {
	content := ""
	if includeText {
		content = fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text)
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, 1, len(objects)+1)
	for i, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xrefOffset := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xrefOffset)
	return b.Bytes()
}

func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

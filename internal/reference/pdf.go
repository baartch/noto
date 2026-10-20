package reference

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

func extractPDF(ctx context.Context, data []byte) (text string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			text = ""
			err = fmt.Errorf("%w: malformed PDF", ErrUnreadable)
		}
	}()
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("%w: malformed or encrypted PDF", ErrUnreadable)
	}
	pages := reader.NumPage()
	if pages <= 0 {
		return "", ErrNoText
	}
	if pages > MaxPDFPages {
		return "", fmt.Errorf("%w: PDF page limit exceeded", ErrTooLarge)
	}
	var out strings.Builder
	for page := 1; page <= pages; page++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		pageText, pageErr := reader.Page(page).GetPlainText(nil)
		if pageErr != nil {
			return "", fmt.Errorf("%w: PDF text extraction failed", ErrUnreadable)
		}
		if out.Len()+len(pageText) > MaxExtractedText {
			return "", ErrTooLarge
		}
		out.WriteString(pageText)
	}
	if strings.TrimSpace(out.String()) == "" {
		return "", ErrNoText
	}
	if !utf8.ValidString(out.String()) {
		return "", errors.Join(ErrUnreadable, errors.New("PDF reader returned invalid UTF-8"))
	}
	return out.String(), nil
}

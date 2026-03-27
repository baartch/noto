package profile

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ConfirmDeletion reads a line from r and returns true only if the user typed "yes".
// It writes the prompt to w.
func ConfirmDeletion(w io.Writer, r io.Reader, profileName string) bool {
	fmt.Fprintf(w, "Type \"yes\" to permanently delete profile %q and all its data: ", profileName)
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return strings.TrimSpace(strings.ToLower(scanner.Text())) == "yes"
	}
	return false
}

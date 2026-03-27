package security

import (
	"fmt"
	"os"
)

// MachinePassphrase returns a stable per-user passphrase derived from the
// machine hostname and OS username. This is used as the AES key derivation
// input for credential encryption at rest.
//
// Note: this does not provide strong multi-user isolation — it keeps secrets
// out of plain-text storage. A future version should use the OS keychain.
func MachinePassphrase() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME") // Windows fallback
	}
	if user == "" {
		user = "noto"
	}
	return fmt.Sprintf("noto:%s:%s", hostname, user), nil
}

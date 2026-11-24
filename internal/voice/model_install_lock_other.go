//go:build !linux

package voice

import "context"

// Other platforms do not currently run the local native runtime. Downloads
// remain unavailable there, so skip both inter-process cleanup and locking.
func acquireModelInstallLock(context.Context, string) (func(), bool, error) {
	return func() {}, false, nil
}

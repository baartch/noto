//go:build linux

package voice

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// acquireModelInstallLock serializes staging cleanup and promotion between
// processes. The lock file is intentionally persistent: unlinking it could
// split waiters across different inodes.
func acquireModelInstallLock(ctx context.Context, modelsDir string) (func(), bool, error) {
	dirInfo, err := os.Lstat(modelsDir)
	if err != nil {
		return nil, false, err
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return nil, false, errors.New("voice: unsafe local model directory for install lock")
	}
	lockPath := filepath.Join(modelsDir, modelInstallLockName)
	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, false, errors.New("voice: could not open local model install lock")
	}
	var fileInfo unix.Stat_t
	err = unix.Fstat(fd, &fileInfo)
	if err != nil || fileInfo.Mode&unix.S_IFMT != unix.S_IFREG {
		_ = unix.Close(fd)
		return nil, false, errors.New("voice: unsafe local model install lock")
	}

	for {
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			if ctx.Err() != nil {
				_ = unix.Flock(fd, unix.LOCK_UN)
				_ = unix.Close(fd)
				return nil, false, ctx.Err()
			}
			return func() {
				_ = unix.Flock(fd, unix.LOCK_UN)
				_ = unix.Close(fd)
			}, true, nil
		}
		if err != unix.EWOULDBLOCK && err != unix.EAGAIN && err != unix.EINTR {
			_ = unix.Close(fd)
			return nil, false, errors.New("voice: could not acquire local model install lock")
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			_ = unix.Close(fd)
			return nil, false, ctx.Err()
		case <-timer.C:
		}
	}
}

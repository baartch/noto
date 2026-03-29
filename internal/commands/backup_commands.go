package commands

import (
	"errors"
	"fmt"

	"noto/internal/backup"
)

// ErrOpenBackupPicker is a sentinel error indicating the TUI should open the backup picker.
type ErrOpenBackupPicker struct{}

func (e *ErrOpenBackupPicker) Error() string { return "open-backup-picker" }

// AsErrOpenBackupPicker checks whether err is an *ErrOpenBackupPicker.
func AsErrOpenBackupPicker(err error) bool {
	_, ok := err.(*ErrOpenBackupPicker)
	return ok
}

// RegisterBackupCommands registers backup-related commands into r.
func RegisterBackupCommands(r *Registry) error {
	return r.Register(&Command{
		Path:        "backup restore",
		Usage:       "backup restore [timestamp]",
		Description: "Restore a profile backup (opens picker if no timestamp)",
		Scope:       ScopeProfile,
		Handler:     backupRestoreHandler,
	})
}

func backupRestoreHandler(ctx *ExecContext, args []string) error {
	if ctx.ProfileSlug == "" {
		return errors.New("no active profile")
	}
	if len(args) == 0 {
		return &ErrOpenBackupPicker{}
	}
	timestamp := args[0]
	if err := backup.RestoreAt(ctx.ProfileSlug, timestamp); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(ctx.Output, "Restored backup %s. Restart chat to reload data.\n", timestamp); err != nil {
		return err
	}
	return nil
}

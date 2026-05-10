package observe

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// FileLogger writes events and messages to a file.
type FileLogger struct {
	file        *os.File
	mu          sync.Mutex
	path        string
	maxSize     int64 // max size in bytes before rotation
	currentSize int64
}

// NewFileLogger creates a new FileLogger that writes to the given path.
func NewFileLogger(path string) (*FileLogger, error) {
	// Ensure directory exists
	dir := fmt.Sprintf("%v/.noto", os.Getenv("HOME"))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("observe: create log dir: %w", err)
	}

	logPath := dir + "/noto.log"
	if path != "" {
		logPath = path
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("observe: open log file: %w", err)
	}

	// Get current file size
	stat, _ := file.Stat()
	currentSize := int64(0)
	if stat != nil {
		currentSize = stat.Size()
	}

	return &FileLogger{
		file:        file,
		path:        logPath,
		maxSize:     10 * 1024 * 1024, // 10MB
		currentSize: currentSize,
	}, nil
}

// Close closes the log file.
func (fl *FileLogger) Close() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.file != nil {
		return fl.file.Close()
	}
	return nil
}

// Emit records a structured event.
func (fl *FileLogger) Emit(e Event) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.file == nil {
		return
	}

	// Rotate if needed
	if fl.currentSize > fl.maxSize {
		fl.rotateLog()
	}

	msg := fmt.Sprintf("[%s] [%s] %s\n", time.Now().Format("15:04:05"), e.EventType, e.Status)
	n, _ := fl.file.WriteString(msg)
	fl.currentSize += int64(n)
}

// Infof logs a free-form informational message.
func (fl *FileLogger) Infof(format string, args ...any) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.file == nil {
		return
	}

	// Rotate if needed
	if fl.currentSize > fl.maxSize {
		fl.rotateLog()
	}

	msg := fmt.Sprintf("[%s] [INFO] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	n, _ := fl.file.WriteString(msg)
	fl.currentSize += int64(n)
}

// Errorf logs a free-form error message.
func (fl *FileLogger) Errorf(format string, args ...any) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if fl.file == nil {
		return
	}

	// Rotate if needed
	if fl.currentSize > fl.maxSize {
		fl.rotateLog()
	}

	msg := fmt.Sprintf("[%s] [ERROR] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
	n, _ := fl.file.WriteString(msg)
	fl.currentSize += int64(n)
}

// rotateLog rotates the log file (called while holding the mutex).
func (fl *FileLogger) rotateLog() {
	if fl.file == nil {
		return
	}

	_ = fl.file.Close()

	// Rename current log to backup
	backupPath := fmt.Sprintf("%s.%d", fl.path, time.Now().Unix())
	_ = os.Rename(fl.path, backupPath)

	// Open a new log file
	file, err := os.OpenFile(fl.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}

	fl.file = file
	fl.currentSize = 0
}

// FileLoggerWithStdout wraps a FileLogger and also writes to stdout.
type FileLoggerWithStdout struct {
	fileLogger *FileLogger
	stdout     Logger
}

// NewFileLoggerWithStdout creates a FileLogger that also writes to stdout.
func NewFileLoggerWithStdout(path string, stdoutLogger Logger) (*FileLoggerWithStdout, error) {
	fileLogger, err := NewFileLogger(path)
	if err != nil {
		return nil, err
	}
	if stdoutLogger == nil {
		stdoutLogger = &NoopLogger{}
	}
	return &FileLoggerWithStdout{
		fileLogger: fileLogger,
		stdout:     stdoutLogger,
	}, nil
}

// Close closes the file logger.
func (flso *FileLoggerWithStdout) Close() error {
	return flso.fileLogger.Close()
}

// Emit records a structured event to both file and stdout.
func (flso *FileLoggerWithStdout) Emit(e Event) {
	flso.fileLogger.Emit(e)
	flso.stdout.Emit(e)
}

// Infof logs to both file and stdout.
func (flso *FileLoggerWithStdout) Infof(format string, args ...any) {
	flso.fileLogger.Infof(format, args...)
	flso.stdout.Infof(format, args...)
}

// Errorf logs to both file and stdout.
func (flso *FileLoggerWithStdout) Errorf(format string, args ...any) {
	flso.fileLogger.Errorf(format, args...)
	flso.stdout.Errorf(format, args...)
}

// NoopLogger is a logger that does nothing.
type NoopLogger struct{}

// Emit is a no-op.
func (nl *NoopLogger) Emit(e Event) {}

// Infof is a no-op.
func (nl *NoopLogger) Infof(format string, args ...any) {}

// Errorf is a no-op.
func (nl *NoopLogger) Errorf(format string, args ...any) {}

package voice

import (
	"archive/tar"
	"compress/bzip2"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"noto/internal/config"
)

const (
	// LocalModelID is the stable profile-selection ID for the bundled Parakeet model.
	LocalModelID            = "parakeet-tdt-0.6b-v3-int8"
	localModelDirectoryName = LocalModelID
	parakeetV3ArchiveRoot   = "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8"
	parakeetV3ArchiveURL    = "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2"
	parakeetV3DownloadBytes = int64(487_211_443)
	maxModelArchiveBytes    = int64(600_000_000)
	maxModelExpandedBytes   = int64(700 << 20)
	maxIgnoredArchiveEntry  = int64(2 << 20)
	maxModelArchiveEntries  = 10_000
	modelInstallLockName    = ".parakeet-install.lock"
	modelDownloadTimeout    = 45 * time.Minute
)

var (
	// ErrLocalRuntimeUnavailable is returned when this build cannot run sherpa-onnx.
	ErrLocalRuntimeUnavailable = errors.New("voice: local speech runtime unavailable")
	// ErrLocalModelNotInstalled is returned when Parakeet is not ready for inference.
	ErrLocalModelNotInstalled = errors.New("voice: local speech model is not installed")
	// ErrLocalModelIntegrity is returned when an installed Parakeet file fails its pinned hash.
	ErrLocalModelIntegrity = errors.New("voice: local speech model integrity check failed")
	// ErrLocalModelDeleting is returned when a local inference request races model deletion.
	ErrLocalModelDeleting = errors.New("voice: local speech model is being deleted")
	// ErrUnsafeModelArchive is returned when a downloaded archive has unsafe entries.
	ErrUnsafeModelArchive = errors.New("voice: unsafe Parakeet model archive")
	// errLocalRecognizerClose is returned when the native recognizer could not be closed.
	errLocalRecognizerClose = errors.New("voice: could not close local speech model")
)

type modelFileSpec struct {
	Name   string
	Size   int64
	SHA256 string
}

type modelPaths struct {
	Dir     string
	Encoder string
	Decoder string
	Joiner  string
	Tokens  string
}

type localRecognizer interface {
	Transcribe(context.Context, []float32) (string, error)
	// Close must be idempotent because a failed close may be retried before reuse or deletion.
	Close() error
}

type modelManagerOptions struct {
	modelsDir            string
	archiveURL           string
	archiveRoot          string
	expectedDownloadSize int64
	maxArchiveBytes      int64
	maxExpandedBytes     int64
	manifest             []modelFileSpec
	testWAVs             map[string]int64
	client               *http.Client
	decompress           func(io.Reader) io.Reader
	runtimeAvailable     bool
	runtimeError         string
	recognizerFactory    func(modelPaths) (localRecognizer, error)
}

// ModelStatus describes readiness and progress for the shared local Parakeet model.
type ModelStatus struct {
	Available          bool    `json:"available"`
	Installed          bool    `json:"installed"`
	Verified           bool    `json:"verified"`
	Verifying          bool    `json:"verifying"`
	Downloading        bool    `json:"downloading"`
	Deleting           bool    `json:"deleting"`
	DownloadedBytes    int64   `json:"downloaded_bytes"`
	DownloadSizeBytes  int64   `json:"download_size_bytes"`
	InstalledSizeBytes int64   `json:"installed_size_bytes"`
	Progress           float64 `json:"progress"`
	Error              string  `json:"error,omitempty"`
}

// ModelManager manages the one shared Noto Parakeet installation.
type ModelManager struct {
	mu             sync.RWMutex
	operationMu    sync.Mutex
	inferenceGate  chan struct{}
	verificationMu sync.Mutex

	modelsDir            string
	modelDir             string
	archiveURL           string
	archiveRoot          string
	expectedDownloadSize int64
	maxArchiveBytes      int64
	maxExpandedBytes     int64
	manifest             []modelFileSpec
	testWAVs             map[string]int64
	client               *http.Client
	decompress           func(io.Reader) io.Reader
	runtimeAvailable     bool
	recognizerFactory    func(modelPaths) (localRecognizer, error)
	recognizer           localRecognizer
	verifiedFiles        []os.FileInfo
	status               ModelStatus
}

// NewModelManager creates a manager for the shared Parakeet v3 model directory.
// It checks local files only; it never contacts the network.
func NewModelManager() *ModelManager {
	appDir, err := config.AppDir()
	if err != nil {
		return newModelManager(modelManagerOptions{runtimeError: err.Error()})
	}
	return newModelManager(modelManagerOptions{
		modelsDir:            filepath.Join(appDir, "models", "local-speech"),
		archiveURL:           parakeetV3ArchiveURL,
		archiveRoot:          parakeetV3ArchiveRoot,
		expectedDownloadSize: parakeetV3DownloadBytes,
		maxArchiveBytes:      maxModelArchiveBytes,
		maxExpandedBytes:     maxModelExpandedBytes,
		manifest:             parakeetV3Manifest(),
		testWAVs: map[string]int64{
			"de.wav": 1 << 20,
			"en.wav": 1 << 20,
			"es.wav": 1 << 20,
			"fr.wav": 1 << 20,
		},
		client:            newModelDownloadClient(),
		decompress:        bzip2.NewReader,
		runtimeAvailable:  localRuntimeAvailable(),
		runtimeError:      localRuntimeUnavailableReason(),
		recognizerFactory: newSherpaRecognizer,
	})
}

func parakeetV3Manifest() []modelFileSpec {
	return []modelFileSpec{
		{Name: "decoder.int8.onnx", Size: 11_845_274, SHA256: "0222a86147299d4dee90edf95b6d13827edb42ad8b7c9cf0b2582f030937c0b5"},
		{Name: "encoder.int8.onnx", Size: 652_282_294, SHA256: "1afbe69c1ffdabee94755f42e4daf1d93549131a3da11356bda84fcef0f2c479"},
		{Name: "joiner.int8.onnx", Size: 6_355_277, SHA256: "cbd02f3696e9944970111fb439d9c3fd365a80374565f13f2b9c7007d253fc98"},
		{Name: "tokens.txt", Size: 93_939, SHA256: "d58544679ea4bc6ac563d1f545eb7d474bd6cfa467f0a6e2c1dc1c7d37e3c35d"},
	}
}

func newModelManager(options modelManagerOptions) *ModelManager {
	if !options.runtimeAvailable && options.runtimeError == "" {
		options.runtimeError = ErrLocalRuntimeUnavailable.Error()
	}
	manifest := append([]modelFileSpec(nil), options.manifest...)
	testWAVs := make(map[string]int64, len(options.testWAVs))
	maps.Copy(testWAVs, options.testWAVs)
	if options.archiveRoot == "" {
		options.archiveRoot = parakeetV3ArchiveRoot
	}
	if options.maxArchiveBytes <= 0 {
		options.maxArchiveBytes = maxModelArchiveBytes
	}
	if options.maxExpandedBytes <= 0 {
		options.maxExpandedBytes = maxModelExpandedBytes
	}
	if options.client == nil {
		options.client = newModelDownloadClient()
	}
	if options.decompress == nil {
		options.decompress = bzip2.NewReader
	}
	m := &ModelManager{
		inferenceGate:        make(chan struct{}, 1),
		modelsDir:            options.modelsDir,
		modelDir:             filepath.Join(options.modelsDir, localModelDirectoryName),
		archiveURL:           options.archiveURL,
		archiveRoot:          options.archiveRoot,
		expectedDownloadSize: options.expectedDownloadSize,
		maxArchiveBytes:      options.maxArchiveBytes,
		maxExpandedBytes:     options.maxExpandedBytes,
		manifest:             manifest,
		testWAVs:             testWAVs,
		client:               options.client,
		decompress:           options.decompress,
		runtimeAvailable:     options.runtimeAvailable,
		recognizerFactory:    options.recognizerFactory,
		status: ModelStatus{
			Available:          options.runtimeAvailable,
			DownloadSizeBytes:  options.expectedDownloadSize,
			InstalledSizeBytes: manifestSize(manifest),
			Error:              options.runtimeError,
		},
	}
	if m.modelsDir == "" {
		m.status.Available = false
		m.status.Error = "voice: local model directory unavailable"
		return m
	}
	installed, err := inspectModelDirectory(m.modelDir, m.manifest)
	if err != nil {
		m.status.Error = err.Error()
	}
	if !installed {
		return m
	}
	m.status.Installed = true
	return m
}

func (m *ModelManager) lockInference(ctx context.Context) error {
	select {
	case m.inferenceGate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			m.unlockInference()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *ModelManager) unlockInference() {
	<-m.inferenceGate
}

func manifestSize(manifest []modelFileSpec) int64 {
	var size int64
	for _, file := range manifest {
		size += file.Size
	}
	return size
}

func (m *ModelManager) paths(dir string) modelPaths {
	return modelPaths{
		Dir:     dir,
		Encoder: filepath.Join(dir, "encoder.int8.onnx"),
		Decoder: filepath.Join(dir, "decoder.int8.onnx"),
		Joiner:  filepath.Join(dir, "joiner.int8.onnx"),
		Tokens:  filepath.Join(dir, "tokens.txt"),
	}
}

// Status returns the current local model state.
func (m *ModelManager) Status() ModelStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// LocalTranscriber returns a local-only transcriber when the runtime and model are ready.
func (m *ModelManager) LocalTranscriber() (Transcriber, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.status.Available {
		return nil, ErrLocalRuntimeUnavailable
	}
	if m.status.Deleting {
		return nil, ErrLocalModelDeleting
	}
	if !m.status.Installed {
		return nil, ErrLocalModelNotInstalled
	}
	return localModelTranscriber{manager: m}, nil
}

type localModelTranscriber struct {
	manager *ModelManager
}

func (t localModelTranscriber) Transcribe(ctx context.Context, data []byte) (string, error) {
	if err := ValidateWAV(data); err != nil {
		return "", err
	}
	m := t.manager
	if err := m.lockInference(ctx); err != nil {
		return "", err
	}
	defer m.unlockInference()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m.mu.RLock()
	if m.status.Deleting {
		m.mu.RUnlock()
		return "", ErrLocalModelDeleting
	}
	if !m.status.Available {
		m.mu.RUnlock()
		return "", ErrLocalRuntimeUnavailable
	}
	if !m.status.Installed {
		m.mu.RUnlock()
		return "", ErrLocalModelNotInstalled
	}
	m.mu.RUnlock()
	samples, err := pcm16WAVSamples(data)
	if err != nil {
		return "", err
	}
	if err := m.ensureVerifiedModelSnapshot(ctx); err != nil {
		return "", err
	}
	m.mu.RLock()
	recognizer := m.recognizer
	factory := m.recognizerFactory
	paths := m.paths(m.modelDir)
	m.mu.RUnlock()
	if recognizer == nil {
		if factory == nil {
			return "", ErrLocalRuntimeUnavailable
		}
		var err error
		recognizer, err = factory(paths)
		if err != nil {
			m.mu.Lock()
			m.status.Error = fmt.Sprintf("voice: could not open local speech model: %v", err)
			m.mu.Unlock()
			return "", errors.New("voice: could not open local speech model")
		}
		m.mu.Lock()
		m.recognizer = recognizer
		m.status.Error = ""
		m.mu.Unlock()
		if err := m.checkRecognizerModelSnapshot(); err != nil {
			return "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text, err := recognizer.Transcribe(ctx, samples)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return text, nil
}

func pcm16WAVSamples(data []byte) ([]float32, error) {
	if err := ValidateWAV(data); err != nil {
		return nil, err
	}
	samples := make([]float32, (len(data)-44)/2)
	for i := range samples {
		value := int16(uint16(data[44+i*2]) | uint16(data[45+i*2])<<8)
		samples[i] = float32(value) / 32768
	}
	return samples, nil
}

// Download explicitly installs the verified Parakeet model. It performs no
// network requests during construction or readiness checks.
func (m *ModelManager) Download(ctx context.Context) (retErr error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	status := m.Status()
	if !status.Available || m.recognizerFactory == nil {
		return ErrLocalRuntimeUnavailable
	}
	if status.Installed {
		if status.Verified {
			return nil
		}
		if err := m.lockInference(ctx); err != nil {
			return err
		}
		err := m.verifyInstalledModel(ctx)
		m.unlockInference()
		if err == nil {
			return nil
		} else if !errors.Is(err, ErrLocalModelIntegrity) && !errors.Is(err, ErrLocalModelNotInstalled) {
			return err
		}
	}
	m.mu.Lock()
	if !m.status.Available || m.recognizerFactory == nil {
		m.mu.Unlock()
		return ErrLocalRuntimeUnavailable
	}
	m.status.Downloading = true
	m.status.Error = ""
	m.status.DownloadedBytes = 0
	m.status.Progress = 0
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.status.Downloading = false
		if retErr != nil {
			m.status.Error = retErr.Error()
		}
		m.mu.Unlock()
	}()

	if err := validateModelDownloadURL(m.archiveURL); err != nil {
		return err
	}
	if err := ensureModelDirectory(m.modelsDir); err != nil {
		return err
	}
	releaseInstallLock, canSweep, err := acquireModelInstallLock(ctx, m.modelsDir)
	if err != nil {
		return err
	}
	defer releaseInstallLock()
	if canSweep {
		if err := sweepOrphanModelStagingDirectories(m.modelsDir); err != nil {
			return fmt.Errorf("voice: clean orphaned model staging directories: %w", err)
		}
	}
	stagingDir, err := os.MkdirTemp(m.modelsDir, ".parakeet-install-v2-*")
	if err != nil {
		return fmt.Errorf("voice: create model staging directory: %w", err)
	}
	keepStage := false
	defer func() {
		if !keepStage {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.archiveURL, nil)
	if err != nil {
		return errors.New("voice: create model download request")
	}
	resp, err := m.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("voice: Parakeet model download failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("voice: Parakeet model download returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > m.maxArchiveBytes {
		return errors.New("voice: Parakeet model archive exceeds download limit")
	}
	total := resp.ContentLength
	if total <= 0 {
		total = m.expectedDownloadSize
	}
	m.mu.Lock()
	if total > 0 {
		m.status.DownloadSizeBytes = total
	}
	m.mu.Unlock()
	progressBody := &modelProgressReader{reader: resp.Body, manager: m, limit: m.maxArchiveBytes, total: total}
	archiveReader := m.decompress(progressBody)
	stagingFiles, err := m.extractAndVerifyWithSnapshot(ctx, archiveReader, stagingDir)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.installStagingDirectory(ctx, stagingDir, stagingFiles); err != nil {
		return err
	}
	keepStage = true // Rename consumed the staging directory.
	return nil
}

func (m *ModelManager) extractAndVerify(ctx context.Context, source io.Reader, stagingDir string) error {
	reader := tar.NewReader(&modelExpandedReader{reader: source, limit: m.maxExpandedBytes})
	manifest := make(map[string]modelFileSpec, len(m.manifest))
	for _, file := range m.manifest {
		manifest[file.Name] = file
	}
	seen := make(map[string]bool, len(manifest))
	seenEntries := make(map[string]bool)
	var expanded int64
	entryCount := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("voice: read Parakeet archive: %w", err)
		}
		entryCount++
		if entryCount > maxModelArchiveEntries {
			return fmt.Errorf("%w: too many archive entries", ErrUnsafeModelArchive)
		}
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			if err := validateHarmlessPAXRecords(header.PAXRecords); err != nil {
				return err
			}
			continue
		}
		name, err := safeModelArchiveName(header.Name)
		if err != nil {
			return err
		}
		if seenEntries[name] {
			return fmt.Errorf("%w: duplicate path", ErrUnsafeModelArchive)
		}
		seenEntries[name] = true
		if header.Size < 0 || expanded+header.Size > m.maxExpandedBytes {
			return errors.New("voice: Parakeet model archive exceeds extraction limit")
		}
		expanded += header.Size
		if header.Typeflag == tar.TypeDir {
			if name != m.archiveRoot && !strings.HasPrefix(name, m.archiveRoot+"/") {
				return fmt.Errorf("%w: unexpected directory", ErrUnsafeModelArchive)
			}
			if header.Size != 0 {
				return fmt.Errorf("%w: non-empty directory entry", ErrUnsafeModelArchive)
			}
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return fmt.Errorf("%w: links and special files are not allowed", ErrUnsafeModelArchive)
		}
		if name == "" || strings.Contains(name, "\\") {
			return fmt.Errorf("%w: invalid archive path", ErrUnsafeModelArchive)
		}
		prefix := m.archiveRoot + "/"
		if !strings.HasPrefix(name, prefix) {
			return fmt.Errorf("%w: unexpected archive path", ErrUnsafeModelArchive)
		}
		relative := strings.TrimPrefix(name, prefix)
		if spec, ok := manifest[relative]; ok {
			if header.Size != spec.Size {
				return fmt.Errorf("voice: Parakeet file %s has unexpected size", spec.Name)
			}
			if err := m.extractVerifiedFile(reader, stagingDir, spec); err != nil {
				return err
			}
			seen[spec.Name] = true
			continue
		}
		if size, ok := m.testWAVs[strings.TrimPrefix(relative, "test_wavs/")]; ok && strings.HasPrefix(relative, "test_wavs/") {
			if header.Size > size {
				return fmt.Errorf("%w: test WAV exceeds size limit", ErrUnsafeModelArchive)
			}
			if _, err := io.CopyN(io.Discard, reader, header.Size); err != nil {
				return fmt.Errorf("voice: read model test WAV: %w", err)
			}
			continue
		}
		if header.Size > maxIgnoredArchiveEntry {
			return fmt.Errorf("%w: unexpected file exceeds ignore limit", ErrUnsafeModelArchive)
		}
		if _, err := io.CopyN(io.Discard, reader, header.Size); err != nil {
			return fmt.Errorf("voice: read ignored archive entry: %w", err)
		}
	}
	for name := range manifest {
		if !seen[name] {
			return fmt.Errorf("voice: Parakeet archive is missing %s", name)
		}
	}
	return nil
}

func (m *ModelManager) extractAndVerifyWithSnapshot(ctx context.Context, source io.Reader, stagingDir string) ([]os.FileInfo, error) {
	if err := m.extractAndVerify(ctx, source, stagingDir); err != nil {
		return nil, err
	}
	files, present, err := snapshotModelDirectory(stagingDir, m.manifest)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, ErrLocalModelIntegrity
	}
	return files, nil
}

func sweepOrphanModelStagingDirectories(modelsDir string) error {
	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".parakeet-install-v2-") {
			continue
		}
		path := filepath.Join(modelsDir, entry.Name())
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

type modelExpandedReader struct {
	reader io.Reader
	limit  int64
	read   int64
}

func (r *modelExpandedReader) Read(p []byte) (int, error) {
	remaining := r.limit - r.read
	if remaining < 0 {
		return 0, errors.New("voice: Parakeet model archive exceeds extraction limit")
	}
	if int64(len(p)) > remaining+1 {
		p = p[:remaining+1]
	}
	n, err := r.reader.Read(p)
	r.read += int64(n)
	if r.read > r.limit {
		return n, errors.New("voice: Parakeet model archive exceeds extraction limit")
	}
	return n, err
}

func safeModelArchiveName(name string) (string, error) {
	if name == "" || strings.Contains(name, "\\") || path.IsAbs(name) {
		return "", fmt.Errorf("%w: invalid archive path", ErrUnsafeModelArchive)
	}
	if slices.Contains(strings.Split(name, "/"), "..") {
		return "", fmt.Errorf("%w: path traversal", ErrUnsafeModelArchive)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: path traversal", ErrUnsafeModelArchive)
	}
	return clean, nil
}

func validateHarmlessPAXRecords(records map[string]string) error {
	totalBytes := 0
	for key, value := range records {
		if key == "path" || key == "linkpath" || key == "size" || strings.HasPrefix(key, "GNU.sparse") {
			return fmt.Errorf("%w: unsafe PAX metadata", ErrUnsafeModelArchive)
		}
		totalBytes += len(key) + len(value)
		if int64(totalBytes) > maxIgnoredArchiveEntry {
			return fmt.Errorf("%w: PAX metadata exceeds ignore limit", ErrUnsafeModelArchive)
		}
	}
	return nil
}

func (m *ModelManager) extractVerifiedFile(reader *tar.Reader, stagingDir string, spec modelFileSpec) error {
	file, err := os.OpenFile(filepath.Join(stagingDir, spec.Name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("voice: create staged model file: %w", err)
	}
	hash := sha256.New()
	written, copyErr := io.CopyN(io.MultiWriter(file, hash), reader, spec.Size)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("voice: extract model file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("voice: close staged model file: %w", closeErr)
	}
	if written != spec.Size || hex.EncodeToString(hash.Sum(nil)) != spec.SHA256 {
		return fmt.Errorf("voice: Parakeet file %s failed SHA-256 verification", spec.Name)
	}
	return nil
}

func (m *ModelManager) installStagingDirectory(ctx context.Context, stagingDir string, stagingFiles []os.FileInfo) error {
	if err := m.lockInference(ctx); err != nil {
		return err
	}
	defer m.unlockInference()
	currentStage, stagePresent, err := snapshotModelDirectory(stagingDir, m.manifest)
	if err != nil || !stagePresent || !sameModelFileSnapshots(stagingFiles, currentStage) {
		return ErrLocalModelIntegrity
	}
	target := m.modelDir
	targetInfo, targetErr := os.Lstat(target)
	if targetErr == nil && (targetInfo.Mode()&os.ModeSymlink != 0 || !targetInfo.IsDir()) {
		return errors.New("voice: unsafe existing model path")
	}
	if targetErr != nil && !errors.Is(targetErr, os.ErrNotExist) {
		return fmt.Errorf("voice: inspect existing model directory: %w", targetErr)
	}
	if err := m.closeCachedRecognizer(); err != nil {
		return err
	}
	currentStage, stagePresent, err = snapshotModelDirectory(stagingDir, m.manifest)
	if err != nil || !stagePresent || !sameModelFileSnapshots(stagingFiles, currentStage) {
		return ErrLocalModelIntegrity
	}
	m.mu.Lock()
	m.status.Installed = false
	m.status.Verified = false
	m.status.Verifying = false
	m.status.Progress = 0
	m.verifiedFiles = nil
	m.mu.Unlock()
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("voice: unsafe existing model path")
		}
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("voice: remove invalid model directory: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("voice: inspect existing model directory: %w", err)
	}
	if err := os.Rename(stagingDir, target); err != nil {
		return fmt.Errorf("voice: install model directory: %w", err)
	}
	verifiedFiles, present, err := snapshotModelDirectory(target, m.manifest)
	if err != nil || !present || !sameModelFileSnapshots(stagingFiles, verifiedFiles) {
		m.mu.Lock()
		m.status.Installed = false
		m.status.Verified = false
		m.status.Error = ErrLocalModelIntegrity.Error()
		m.mu.Unlock()
		return ErrLocalModelIntegrity
	}
	m.mu.Lock()
	m.status.Installed = true
	m.status.Verified = true
	m.status.Verifying = false
	m.status.InstalledSizeBytes = manifestSize(m.manifest)
	m.status.Progress = 1
	m.status.Error = ""
	m.verifiedFiles = verifiedFiles
	m.mu.Unlock()
	return nil
}

// Delete removes the shared model after any active inference has completed.
func (m *ModelManager) Delete(ctx context.Context) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	m.status.Deleting = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.status.Deleting = false
		m.mu.Unlock()
	}()
	if err := m.lockInference(ctx); err != nil {
		return err
	}
	defer m.unlockInference()

	if err := m.closeCachedRecognizer(); err != nil {
		return fmt.Errorf("voice: close local speech model: %w", err)
	}
	m.mu.Lock()
	m.verifiedFiles = nil
	m.status.Installed = false
	m.status.Verified = false
	m.status.Verifying = false
	m.status.DownloadedBytes = 0
	m.status.Progress = 0
	m.mu.Unlock()
	if info, err := os.Lstat(m.modelDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			m.mu.Lock()
			m.status.Error = "voice: unsafe model directory path"
			m.mu.Unlock()
			return errors.New("voice: unsafe model directory path")
		}
		if err := os.RemoveAll(m.modelDir); err != nil {
			m.mu.Lock()
			m.status.Error = "voice: could not delete local model"
			m.mu.Unlock()
			return fmt.Errorf("voice: delete local model: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		m.mu.Lock()
		m.status.Error = "voice: could not inspect local model directory"
		m.mu.Unlock()
		return fmt.Errorf("voice: inspect local model directory: %w", err)
	}
	m.mu.Lock()
	m.status.DownloadSizeBytes = m.expectedDownloadSize
	m.status.InstalledSizeBytes = manifestSize(m.manifest)
	m.status.Error = ""
	m.mu.Unlock()
	return nil
}

func (m *ModelManager) updateProgress(received, total int64) {
	m.mu.Lock()
	m.status.DownloadedBytes = received
	if total > 0 {
		m.status.DownloadSizeBytes = total
		m.status.Progress = float64(received) / float64(total)
		if m.status.Progress > 1 {
			m.status.Progress = 1
		}
	}
	m.mu.Unlock()
}

type modelProgressReader struct {
	reader  io.Reader
	manager *ModelManager
	limit   int64
	total   int64
	seen    int64
}

func (r *modelProgressReader) Read(p []byte) (int, error) {
	remaining := r.limit - r.seen
	if remaining < 0 {
		return 0, errors.New("voice: Parakeet model archive exceeds download limit")
	}
	if int64(len(p)) > remaining+1 {
		p = p[:remaining+1]
	}
	n, err := r.reader.Read(p)
	r.seen += int64(n)
	r.manager.updateProgress(r.seen, r.total)
	if r.seen > r.limit {
		return n, errors.New("voice: Parakeet model archive exceeds download limit")
	}
	return n, err
}

func verifyModelDirectory(dir string, manifest []modelFileSpec) (bool, error) {
	return verifyModelDirectoryContext(context.Background(), dir, manifest)
}

func inspectModelDirectory(dir string, manifest []modelFileSpec) (bool, error) {
	_, present, err := snapshotModelDirectory(dir, manifest)
	return present, err
}

func snapshotModelDirectory(dir string, manifest []modelFileSpec) ([]os.FileInfo, bool, error) {
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, false, errors.New("voice: unsafe installed model directory")
	}
	files := make([]os.FileInfo, 0, len(manifest))
	for _, spec := range manifest {
		path := filepath.Join(dir, spec.Name)
		fileInfo, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, false, nil
			}
			return nil, false, err
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() || fileInfo.Size() != spec.Size {
			return nil, false, nil
		}
		files = append(files, fileInfo)
	}
	return files, true, nil
}

func verifyModelDirectoryContext(ctx context.Context, dir string, manifest []modelFileSpec) (bool, error) {
	_, present, err := verifyModelDirectoryContextWithSnapshot(ctx, dir, manifest)
	return present, err
}

func verifyModelDirectoryContextWithSnapshot(ctx context.Context, dir string, manifest []modelFileSpec) ([]os.FileInfo, bool, error) {
	before, present, err := snapshotModelDirectory(dir, manifest)
	if err != nil || !present {
		return nil, present, err
	}
	for i, spec := range manifest {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		filePath := filepath.Join(dir, spec.Name)
		file, err := os.Open(filePath)
		if err != nil {
			return nil, false, err
		}
		openedInfo, err := file.Stat()
		if err != nil || !sameModelFileInfo(before[i], openedInfo) {
			_ = file.Close()
			if err != nil {
				return nil, false, err
			}
			return nil, false, nil
		}
		hash := sha256.New()
		_, readErr := io.Copy(hash, contextModelReader{ctx: ctx, reader: file})
		finishedInfo, statErr := file.Stat()
		closeErr := file.Close()
		if readErr != nil {
			return nil, false, readErr
		}
		if statErr != nil {
			return nil, false, statErr
		}
		if closeErr != nil {
			return nil, false, closeErr
		}
		if !sameModelFileInfo(before[i], finishedInfo) {
			return nil, false, nil
		}
		if hex.EncodeToString(hash.Sum(nil)) != spec.SHA256 {
			return nil, false, nil
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	after, stillPresent, err := snapshotModelDirectory(dir, manifest)
	if err != nil || !stillPresent || !sameModelFileSnapshots(before, after) {
		return nil, false, err
	}
	return after, true, nil
}

func sameModelFileInfo(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func sameModelFileSnapshots(a, b []os.FileInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameModelFileInfo(a[i], b[i]) {
			return false
		}
	}
	return true
}

func (m *ModelManager) verifyInstalledModel(ctx context.Context) error {
	m.verificationMu.Lock()
	defer m.verificationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	if !m.status.Installed {
		m.mu.Unlock()
		return ErrLocalModelNotInstalled
	}
	if m.status.Verified {
		m.mu.Unlock()
		return nil
	}
	m.status.Verifying = true
	dir := m.modelDir
	manifest := append([]modelFileSpec(nil), m.manifest...)
	m.mu.Unlock()
	if err := m.closeCachedRecognizer(); err != nil {
		m.mu.Lock()
		m.status.Verifying = false
		m.mu.Unlock()
		return err
	}

	verifiedFiles, verified, err := verifyModelDirectoryContextWithSnapshot(ctx, dir, manifest)
	m.mu.Lock()
	m.status.Verifying = false
	if ctx.Err() != nil {
		m.mu.Unlock()
		return ctx.Err()
	}
	if err != nil || !verified {
		m.verifiedFiles = nil
		m.status.Installed = false
		m.status.Verified = false
		m.status.Error = ErrLocalModelIntegrity.Error()
		m.mu.Unlock()
		if closeErr := m.closeCachedRecognizer(); closeErr != nil {
			return closeErr
		}
		m.mu.Lock()
		m.status.Error = ErrLocalModelIntegrity.Error()
		m.mu.Unlock()
		return ErrLocalModelIntegrity
	}
	m.status.Verified = true
	m.status.Error = ""
	m.verifiedFiles = verifiedFiles
	m.mu.Unlock()
	return nil
}

// ensureVerifiedModelSnapshot rehashes pinned files whenever their identity,
// type, size or modification time changes. Metadata is a change detector, not
// an authentication mechanism against an actor able to preserve metadata.
func (m *ModelManager) ensureVerifiedModelSnapshot(ctx context.Context) error {
	m.mu.RLock()
	verified := m.status.Verified
	previous := append([]os.FileInfo(nil), m.verifiedFiles...)
	pendingClose := m.recognizer != nil && !verified
	dir := m.modelDir
	manifest := append([]modelFileSpec(nil), m.manifest...)
	m.mu.RUnlock()
	if pendingClose {
		if err := m.closeCachedRecognizer(); err != nil {
			return err
		}
	}
	if !verified || len(previous) != len(manifest) {
		m.mu.Lock()
		if m.status.Error == ErrLocalModelIntegrity.Error() || m.status.Error == errLocalRecognizerClose.Error() {
			m.status.Error = ""
		}
		m.mu.Unlock()
		return m.verifyInstalledModel(ctx)
	}
	current, present, err := snapshotModelDirectory(dir, manifest)
	if err == nil && present && sameModelFileSnapshots(previous, current) {
		return nil
	}

	m.mu.Lock()
	m.status.Verified = false
	m.verifiedFiles = nil
	m.status.Error = ""
	m.mu.Unlock()
	if closeErr := m.closeCachedRecognizer(); closeErr != nil {
		return closeErr
	}
	if err != nil || !present {
		m.mu.Lock()
		m.status.Installed = false
		m.status.Error = ErrLocalModelIntegrity.Error()
		m.mu.Unlock()
		return ErrLocalModelIntegrity
	}
	return m.verifyInstalledModel(ctx)
}

func (m *ModelManager) checkRecognizerModelSnapshot() error {
	m.mu.RLock()
	expected := append([]os.FileInfo(nil), m.verifiedFiles...)
	dir := m.modelDir
	manifest := append([]modelFileSpec(nil), m.manifest...)
	m.mu.RUnlock()
	current, present, err := snapshotModelDirectory(dir, manifest)
	if err == nil && present && sameModelFileSnapshots(expected, current) {
		return nil
	}
	m.mu.Lock()
	m.verifiedFiles = nil
	m.status.Verified = false
	m.status.Error = ErrLocalModelIntegrity.Error()
	m.mu.Unlock()
	if err := m.closeCachedRecognizer(); err != nil {
		return err
	}
	m.mu.Lock()
	m.status.Error = ErrLocalModelIntegrity.Error()
	m.mu.Unlock()
	return ErrLocalModelIntegrity
}

// closeCachedRecognizer is called while holding the inference gate, so no
// decode can still be using the native recognizer.
func (m *ModelManager) closeCachedRecognizer() error {
	m.mu.RLock()
	recognizer := m.recognizer
	m.mu.RUnlock()
	if recognizer == nil {
		return nil
	}
	if err := recognizer.Close(); err != nil {
		m.mu.Lock()
		m.status.Verified = false
		m.verifiedFiles = nil
		m.status.Error = errLocalRecognizerClose.Error()
		m.mu.Unlock()
		return errLocalRecognizerClose
	}
	m.mu.Lock()
	m.recognizer = nil
	if !m.status.Verified {
		m.status.Error = ""
	}
	m.mu.Unlock()
	return nil
}

type contextModelReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextModelReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func ensureModelDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("voice: create local model directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("voice: inspect local model directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("voice: unsafe local model directory")
	}
	return nil
}

func validateModelDownloadURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return errors.New("voice: invalid Parakeet model URL")
	}
	if strings.EqualFold(u.Scheme, "https") && strings.EqualFold(u.Hostname(), "github.com") {
		return nil
	}
	if strings.EqualFold(u.Scheme, "http") {
		ip := net.ParseIP(u.Hostname())
		if ip != nil && ip.IsLoopback() {
			return nil
		}
	}
	return errors.New("voice: Parakeet downloads require the official HTTPS source")
}

func newModelDownloadClient() *http.Client {
	return &http.Client{
		Timeout: modelDownloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || !strings.EqualFold(req.URL.Scheme, "https") || req.URL.User != nil {
				return errors.New("voice: unsafe model download redirect")
			}
			if port := req.URL.Port(); port != "" && port != "443" {
				return errors.New("voice: unsafe model download redirect")
			}
			host := strings.ToLower(req.URL.Hostname())
			if host != "github.com" && host != "release-assets.githubusercontent.com" {
				return errors.New("voice: unsafe model download redirect")
			}
			return nil
		},
	}
}

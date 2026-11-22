package voice

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testArchiveRoot = "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8"

type testLocalRecognizer struct {
	text       string
	started    chan struct{}
	release    chan struct{}
	startedOne sync.Once
	closed     atomic.Int32
	samples    []float32
}

func (r *testLocalRecognizer) Transcribe(ctx context.Context, samples []float32) (string, error) {
	r.samples = append([]float32(nil), samples...)
	if r.started != nil {
		r.startedOne.Do(func() { close(r.started) })
	}
	if r.release != nil {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-r.release:
		}
	}
	return r.text, nil
}

func (r *testLocalRecognizer) Close() error {
	r.closed.Add(1)
	return nil
}

type modelTestFixture struct {
	manager   *ModelManager
	server    *httptest.Server
	modelsDir string
	requests  *atomic.Int32
	manifest  []modelFileSpec
	contents  map[string][]byte
}

func newModelTestFixture(
	t *testing.T,
	archive []byte,
	manifest []modelFileSpec,
	contents map[string][]byte,
	factory func(modelPaths) (localRecognizer, error),
	runtimeAvailable bool,
	handler http.HandlerFunc,
) *modelTestFixture {
	t.Helper()
	requests := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		_, _ = w.Write(archive)
	}))
	t.Cleanup(server.Close)

	modelsDir := filepath.Join(t.TempDir(), "models")
	manager := newModelManager(modelManagerOptions{
		modelsDir:            modelsDir,
		archiveURL:           server.URL + "/model.tar.bz2",
		archiveRoot:          testArchiveRoot,
		expectedDownloadSize: int64(len(archive)),
		maxArchiveBytes:      4 << 20,
		maxExpandedBytes:     1 << 20,
		manifest:             manifest,
		testWAVs:             map[string]int64{"en.wav": 1 << 20},
		client:               server.Client(),
		decompress:           func(r io.Reader) io.Reader { return r },
		runtimeAvailable:     runtimeAvailable,
		recognizerFactory:    factory,
	})
	return &modelTestFixture{manager: manager, server: server, modelsDir: modelsDir, requests: requests, manifest: manifest, contents: contents}
}

func testModelContents() map[string][]byte {
	return map[string][]byte{
		"encoder.int8.onnx": []byte("encoder model bytes"),
		"decoder.int8.onnx": []byte("decoder model bytes"),
		"joiner.int8.onnx":  []byte("joiner model bytes"),
		"tokens.txt":        []byte("token-a\ntoken-b\n"),
	}
}

func manifestForTestFiles(contents map[string][]byte) []modelFileSpec {
	names := make([]string, 0, len(contents))
	for name := range contents {
		names = append(names, name)
	}
	sort.Strings(names)
	manifest := make([]modelFileSpec, 0, len(names))
	for _, name := range names {
		hash := sha256.Sum256(contents[name])
		manifest = append(manifest, modelFileSpec{Name: name, Size: int64(len(contents[name])), SHA256: hex.EncodeToString(hash[:])})
	}
	return manifest
}

func makeModelArchive(t *testing.T, contents map[string][]byte) []byte {
	return makeModelArchiveWithGlobalPAX(t, contents, nil)
}

func makeModelArchiveWithGlobalPAX(t *testing.T, contents map[string][]byte, paxRecords map[string]string) []byte {
	t.Helper()
	var archive bytes.Buffer
	w := tar.NewWriter(&archive)
	if paxRecords != nil {
		if err := w.WriteHeader(&tar.Header{Name: "GlobalHead.0", Typeflag: tar.TypeXGlobalHeader, Format: tar.FormatPAX, PAXRecords: paxRecords}); err != nil {
			t.Fatalf("write global PAX header: %v", err)
		}
	}
	if err := w.WriteHeader(&tar.Header{Name: testArchiveRoot + "/", Typeflag: tar.TypeDir, Mode: 0o700}); err != nil {
		t.Fatalf("write archive root: %v", err)
	}
	if err := w.WriteHeader(&tar.Header{Name: testArchiveRoot + "/test_wavs/", Typeflag: tar.TypeDir, Mode: 0o700}); err != nil {
		t.Fatalf("write test WAV directory: %v", err)
	}
	testWAV := []byte("bundled test sample")
	if err := w.WriteHeader(&tar.Header{Name: testArchiveRoot + "/test_wavs/en.wav", Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(testWAV))}); err != nil {
		t.Fatalf("write test WAV header: %v", err)
	}
	if _, err := w.Write(testWAV); err != nil {
		t.Fatalf("write test WAV: %v", err)
	}
	names := make([]string, 0, len(contents))
	for name := range contents {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content := contents[name]
		if err := w.WriteHeader(&tar.Header{Name: filepath.ToSlash(filepath.Join(testArchiveRoot, name)), Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatalf("write %s header: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("write %s content: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return archive.Bytes()
}

func makeModelArchiveWithLargePAX(t *testing.T, contents map[string][]byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	w := tar.NewWriter(&archive)
	if err := w.WriteHeader(&tar.Header{Name: testArchiveRoot + "/", Typeflag: tar.TypeDir, Mode: 0o700}); err != nil {
		t.Fatalf("write archive root: %v", err)
	}
	names := make([]string, 0, len(contents))
	for name := range contents {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content := contents[name]
		header := &tar.Header{Name: filepath.ToSlash(filepath.Join(testArchiveRoot, name)), Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(content))}
		if name == "encoder.int8.onnx" {
			header.Format = tar.FormatPAX
			header.PAXRecords = map[string]string{"comment": strings.Repeat("x", 64<<10)}
		}
		if err := w.WriteHeader(header); err != nil {
			t.Fatalf("write %s header: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("write %s content: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return archive.Bytes()
}

func makeUnsafeModelArchive(t *testing.T, header *tar.Header, body []byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	w := tar.NewWriter(&archive)
	if err := w.WriteHeader(&tar.Header{Name: testArchiveRoot + "/", Typeflag: tar.TypeDir, Mode: 0o700}); err != nil {
		t.Fatalf("write archive root: %v", err)
	}
	if err := w.WriteHeader(header); err != nil {
		t.Fatalf("write malicious header: %v", err)
	}
	if len(body) > 0 {
		if _, err := w.Write(body); err != nil {
			t.Fatalf("write malicious body: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return archive.Bytes()
}

func TestModelManagerDownloadsOnlyOnRequestAndSharesInstalledModel(t *testing.T) {
	contents := testModelContents()
	manifest := manifestForTestFiles(contents)
	archive := makeModelArchive(t, contents)
	var factoryCalls atomic.Int32
	factory := func(modelPaths) (localRecognizer, error) {
		factoryCalls.Add(1)
		return &testLocalRecognizer{text: "local transcript"}, nil
	}
	fixture := newModelTestFixture(t, archive, manifest, contents, factory, true, nil)

	status := fixture.manager.Status()
	if !status.Available || status.Installed || status.Downloading {
		t.Fatalf("initial status = %+v, want available and not installed", status)
	}
	if got := fixture.requests.Load(); got != 0 {
		t.Fatalf("initialization made %d HTTP requests, want none", got)
	}
	if got := factoryCalls.Load(); got != 0 {
		t.Fatalf("initialization loaded recognizer %d times, want none", got)
	}
	if _, err := fixture.manager.LocalTranscriber(); !errors.Is(err, ErrLocalModelNotInstalled) {
		t.Fatalf("LocalTranscriber() error = %v, want ErrLocalModelNotInstalled", err)
	}
	if got := fixture.requests.Load(); got != 0 {
		t.Fatalf("readiness check made %d HTTP requests, want none", got)
	}

	if err := fixture.manager.Download(context.Background()); err != nil {
		t.Fatalf("Download(): %v", err)
	}
	status = fixture.manager.Status()
	if !status.Installed || !status.Verified || status.Downloading || status.DownloadedBytes != int64(len(archive)) {
		t.Fatalf("downloaded status = %+v", status)
	}
	if got := factoryCalls.Load(); got != 0 {
		t.Fatalf("download loaded recognizer %d times, want lazy loading", got)
	}
	if status.Progress != 1 {
		t.Fatalf("completed progress = %f, want 1", status.Progress)
	}
	transcriber, err := fixture.manager.LocalTranscriber()
	if err != nil {
		t.Fatalf("LocalTranscriber(): %v", err)
	}
	got, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{0, 128, 255, 127}))
	if err != nil || got != "local transcript" {
		t.Fatalf("local transcription = %q, %v", got, err)
	}
	if got := factoryCalls.Load(); got != 1 {
		t.Fatalf("recognizer factory calls = %d, want one on first transcription", got)
	}
	if got := fixture.requests.Load(); got != 1 {
		t.Fatalf("model download requests = %d, want one", got)
	}
	if err := fixture.manager.Download(context.Background()); err != nil {
		t.Fatalf("repeat Download(): %v", err)
	}
	if got := fixture.requests.Load(); got != 1 {
		t.Fatalf("repeat Download() made %d requests, want no additional download", got-1)
	}

	secondManager := newModelManager(modelManagerOptions{
		modelsDir:            fixture.modelsDir,
		archiveURL:           fixture.server.URL + "/model.tar.bz2",
		expectedDownloadSize: int64(len(archive)),
		maxArchiveBytes:      1 << 20,
		maxExpandedBytes:     1 << 20,
		manifest:             manifest,
		testWAVs:             map[string]int64{},
		client:               fixture.server.Client(),
		decompress:           func(r io.Reader) io.Reader { return r },
		runtimeAvailable:     true,
		recognizerFactory:    factory,
	})
	if !secondManager.Status().Installed || secondManager.Status().Verified {
		t.Fatalf("second manager did not reuse shared install: %+v", secondManager.Status())
	}
	if got := factoryCalls.Load(); got != 1 {
		t.Fatalf("status initialization loaded recognizer; calls = %d, want 1", got)
	}
	secondTranscriber, err := secondManager.LocalTranscriber()
	if err != nil {
		t.Fatalf("second manager LocalTranscriber(): %v", err)
	}
	if _, err := secondTranscriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2})); err != nil {
		t.Fatalf("second manager local transcription: %v", err)
	}
	if !secondManager.Status().Verified || factoryCalls.Load() != 2 {
		t.Fatalf("first local use did not verify/load shared model: status=%+v calls=%d", secondManager.Status(), factoryCalls.Load())
	}
	if got := fixture.requests.Load(); got != 1 {
		t.Fatalf("shared model triggered %d downloads, want one", got)
	}
}

func TestModelManagerIgnoresBoundedDocumentationAndSafePAX(t *testing.T) {
	contents := testModelContents()
	contents["README.md"] = []byte("export notes")
	contents["trans.txt"] = []byte("sample transcript")
	manifest := manifestForTestFiles(contents)
	filteredManifest := make([]modelFileSpec, 0, len(manifest)-2)
	for _, spec := range manifest {
		if spec.Name != "README.md" && spec.Name != "trans.txt" {
			filteredManifest = append(filteredManifest, spec)
		}
	}
	archive := makeModelArchiveWithGlobalPAX(t, contents, map[string]string{"comment": "sherpa export metadata"})
	factory := func(modelPaths) (localRecognizer, error) {
		return &testLocalRecognizer{text: "local"}, nil
	}
	fixture := newModelTestFixture(t, archive, filteredManifest, contents, factory, true, nil)
	if err := fixture.manager.Download(context.Background()); err != nil {
		t.Fatalf("Download() with harmless metadata failed: %v", err)
	}
	installed, err := verifyModelDirectory(filepath.Join(fixture.modelsDir, localModelDirectoryName), filteredManifest)
	if err != nil || !installed {
		t.Fatalf("installed model verification = %t, %v", installed, err)
	}
}

func TestModelManagerRejectsGlobalPAXPathOverride(t *testing.T) {
	contents := testModelContents()
	archive := makeModelArchiveWithGlobalPAX(t, contents, map[string]string{"path": "../outside"})
	fixture := newModelTestFixture(t, archive, manifestForTestFiles(contents), contents, func(modelPaths) (localRecognizer, error) {
		return &testLocalRecognizer{text: "unused"}, nil
	}, true, nil)
	if err := fixture.manager.Download(context.Background()); err == nil {
		t.Fatal("Download() accepted a global PAX path override")
	}
}

func TestModelManagerReportsDownloadProgress(t *testing.T) {
	contents := testModelContents()
	manifest := manifestForTestFiles(contents)
	archive := makeModelArchive(t, contents)
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
		half := len(archive) / 2
		_, _ = w.Write(archive[:half])
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		close(started)
		<-release
		_, _ = w.Write(archive[half:])
	}
	fixture := newModelTestFixture(t, archive, manifest, contents, func(modelPaths) (localRecognizer, error) {
		return &testLocalRecognizer{text: "local"}, nil
	}, true, handler)
	done := make(chan error, 1)
	go func() { done <- fixture.manager.Download(context.Background()) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("download did not start")
	}
	waitForModelStatus(t, fixture.manager, func(s ModelStatus) bool {
		return s.DownloadedBytes > 0 && s.DownloadedBytes < int64(len(archive))
	})
	status := fixture.manager.Status()
	if status.Progress <= 0 || status.Progress >= 1 {
		t.Fatalf("intermediate progress = %f, want between 0 and 1", status.Progress)
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatalf("Download(): %v", err)
	}
}

func TestModelManagerRejectsChecksumFailureAndCleansStaging(t *testing.T) {
	contents := testModelContents()
	manifest := manifestForTestFiles(contents)
	manifest[0].SHA256 = strings.Repeat("0", 64)
	fixture := newModelTestFixture(t, makeModelArchive(t, contents), manifest, contents, func(modelPaths) (localRecognizer, error) {
		return &testLocalRecognizer{text: "unused"}, nil
	}, true, nil)

	err := fixture.manager.Download(context.Background())
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("Download() error = %v, want checksum error", err)
	}
	if fixture.manager.Status().Installed {
		t.Fatalf("checksum failure left model installed: %+v", fixture.manager.Status())
	}
	entries, err := os.ReadDir(fixture.modelsDir)
	if err != nil {
		t.Fatalf("read model dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed install left staging entries: %v", entries)
	}
}

func TestModelManagerRepairsCorruptExistingInstall(t *testing.T) {
	contents := testModelContents()
	manifest := manifestForTestFiles(contents)
	archive := makeModelArchive(t, contents)
	fixture := newModelTestFixture(t, archive, manifest, contents, func(modelPaths) (localRecognizer, error) {
		return &testLocalRecognizer{text: "reinstalled"}, nil
	}, true, nil)
	target := filepath.Join(fixture.modelsDir, localModelDirectoryName)
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("create corrupt model directory: %v", err)
	}
	corruptEncoder := bytes.Repeat([]byte{0xff}, len(contents["encoder.int8.onnx"]))
	for name, content := range contents {
		if name == "encoder.int8.onnx" {
			content = corruptEncoder
		}
		if err := os.WriteFile(filepath.Join(target, name), content, 0o600); err != nil {
			t.Fatalf("write model file %s: %v", name, err)
		}
	}
	fixture.manager = newModelManager(modelManagerOptions{
		modelsDir:            fixture.modelsDir,
		archiveURL:           fixture.server.URL + "/model.tar.bz2",
		archiveRoot:          testArchiveRoot,
		expectedDownloadSize: int64(len(archive)),
		maxArchiveBytes:      4 << 20,
		maxExpandedBytes:     1 << 20,
		manifest:             manifest,
		testWAVs:             map[string]int64{"en.wav": 1 << 20},
		client:               fixture.server.Client(),
		decompress:           func(r io.Reader) io.Reader { return r },
		runtimeAvailable:     true,
		recognizerFactory: func(modelPaths) (localRecognizer, error) {
			return &testLocalRecognizer{text: "repaired"}, nil
		},
	})
	if !fixture.manager.Status().Installed || fixture.manager.Status().Verified {
		t.Fatalf("corrupt same-size startup status = %+v, want present but unverified", fixture.manager.Status())
	}

	if err := fixture.manager.Download(context.Background()); err != nil {
		t.Fatalf("Download() should repair the corrupt install: %v", err)
	}
	installed, err := verifyModelDirectory(target, manifest)
	if err != nil || !installed {
		t.Fatalf("repaired model verification = %t, %v", installed, err)
	}
	entries, err := os.ReadDir(fixture.modelsDir)
	if err != nil {
		t.Fatalf("read model directory: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != localModelDirectoryName {
		t.Fatalf("model directory entries = %v, want only installed model", entries)
	}
}

func TestModelManagerFailedPromotionClearsInstalledStatus(t *testing.T) {
	contents := testModelContents()
	archive := makeModelArchive(t, contents)
	manifest := manifestForTestFiles(contents)
	fixture := newModelTestFixture(t, archive, manifest, contents, func(modelPaths) (localRecognizer, error) {
		return &testLocalRecognizer{text: "local"}, nil
	}, true, nil)
	if err := fixture.manager.Download(context.Background()); err != nil {
		t.Fatalf("initial Download(): %v", err)
	}
	if status := fixture.manager.Status(); !status.Installed || !status.Verified {
		t.Fatalf("initial status = %+v, want installed and verified", status)
	}

	missingStagingDir := filepath.Join(fixture.modelsDir, ".missing-parakeet-stage")
	if err := fixture.manager.installStagingDirectory(context.Background(), missingStagingDir); err == nil {
		t.Fatal("installStagingDirectory() succeeded for a missing stage")
	}
	if status := fixture.manager.Status(); status.Installed || status.Verified {
		t.Fatalf("failed promotion left stale readiness: %+v", status)
	}
	if err := fixture.manager.Download(context.Background()); err != nil {
		t.Fatalf("Download() could not repair after failed promotion: %v", err)
	}
}

func TestModelManagerBoundsDecompressedArchive(t *testing.T) {
	contents := testModelContents()
	fixture := newModelTestFixture(t, makeModelArchiveWithLargePAX(t, contents), manifestForTestFiles(contents), contents, func(modelPaths) (localRecognizer, error) {
		return &testLocalRecognizer{text: "unused"}, nil
	}, true, nil)
	fixture.manager.maxExpandedBytes = 1 << 10
	if err := fixture.manager.Download(context.Background()); err == nil || !strings.Contains(err.Error(), "extraction limit") {
		t.Fatalf("Download() error = %v, want decompressed-size limit error", err)
	}
	if fixture.manager.Status().Installed {
		t.Fatalf("oversized archive marked model installed: %+v", fixture.manager.Status())
	}
	entries, err := os.ReadDir(fixture.modelsDir)
	if err != nil {
		t.Fatalf("read model dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("oversized archive left staging entries: %v", entries)
	}
}

func TestModelManagerCleansInterruptedDownload(t *testing.T) {
	contents := testModelContents()
	manifest := manifestForTestFiles(contents)
	var partial bytes.Buffer
	w := tar.NewWriter(&partial)
	_ = w.WriteHeader(&tar.Header{Name: testArchiveRoot + "/", Typeflag: tar.TypeDir, Mode: 0o700})
	_ = w.WriteHeader(&tar.Header{Name: testArchiveRoot + "/encoder.int8.onnx", Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(contents["encoder.int8.onnx"]))})
	_, _ = w.Write([]byte("partial"))
	fixture := newModelTestFixture(t, partial.Bytes(), manifest, contents, func(modelPaths) (localRecognizer, error) {
		return &testLocalRecognizer{text: "unused"}, nil
	}, true, nil)

	if err := fixture.manager.Download(context.Background()); err == nil {
		t.Fatal("Download() succeeded for a truncated archive")
	}
	if fixture.manager.Status().Installed {
		t.Fatalf("interrupted download marked installed: %+v", fixture.manager.Status())
	}
	entries, err := os.ReadDir(fixture.modelsDir)
	if err != nil {
		t.Fatalf("read model dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("interrupted download left staging entries: %v", entries)
	}
}

func TestModelManagerRejectsRedirectOutsideAllowlist(t *testing.T) {
	var targetRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetRequests.Add(1)
		_, _ = w.Write([]byte("unexpected model archive"))
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/model.tar.bz2", http.StatusFound)
	}))
	defer redirect.Close()

	contents := testModelContents()
	modelsDir := filepath.Join(t.TempDir(), "models")
	manager := newModelManager(modelManagerOptions{
		modelsDir:            modelsDir,
		archiveURL:           redirect.URL + "/model.tar.bz2",
		archiveRoot:          testArchiveRoot,
		expectedDownloadSize: 64,
		maxArchiveBytes:      1 << 20,
		maxExpandedBytes:     1 << 20,
		manifest:             manifestForTestFiles(contents),
		testWAVs:             map[string]int64{},
		client:               newModelDownloadClient(),
		decompress:           func(r io.Reader) io.Reader { return r },
		runtimeAvailable:     true,
		recognizerFactory: func(modelPaths) (localRecognizer, error) {
			return &testLocalRecognizer{text: "unused"}, nil
		},
	})
	if err := manager.Download(context.Background()); err == nil {
		t.Fatal("Download() followed an untrusted redirect")
	}
	if got := targetRequests.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests, want 0", got)
	}
}

func TestValidateModelDownloadURL(t *testing.T) {
	for _, tc := range []struct {
		url     string
		wantErr bool
	}{
		{url: parakeetV3ArchiveURL},
		{url: "http://127.0.0.1:1234/model.tar.bz2"},
		{url: "https://github.com.evil.invalid/model.tar.bz2", wantErr: true},
		{url: "https://user:secret@github.com/model.tar.bz2", wantErr: true},
		{url: "http://models.example.invalid/model.tar.bz2", wantErr: true},
	} {
		t.Run(tc.url, func(t *testing.T) {
			err := validateModelDownloadURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatal("validateModelDownloadURL() succeeded, want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateModelDownloadURL() error = %v", err)
			}
		})
	}
}

func TestModelManagerRejectsTraversalAndSymlinks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header *tar.Header
		body   []byte
	}{
		{
			name:   "traversal",
			header: &tar.Header{Name: "../outside", Typeflag: tar.TypeReg, Mode: 0o600, Size: 4},
			body:   []byte("evil"),
		},
		{
			name:   "symlink",
			header: &tar.Header{Name: testArchiveRoot + "/encoder.int8.onnx", Typeflag: tar.TypeSymlink, Linkname: "../../outside", Mode: 0o777},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contents := testModelContents()
			fixture := newModelTestFixture(t, makeUnsafeModelArchive(t, tc.header, tc.body), manifestForTestFiles(contents), contents, func(modelPaths) (localRecognizer, error) {
				return &testLocalRecognizer{text: "unused"}, nil
			}, true, nil)
			if err := fixture.manager.Download(context.Background()); err == nil {
				t.Fatal("Download() succeeded for an unsafe archive entry")
			}
			outside := filepath.Join(filepath.Dir(fixture.modelsDir), "outside")
			if _, err := os.Lstat(outside); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unsafe archive wrote outside model dir: %v", err)
			}
		})
	}
}

func TestSafeModelArchiveNameAllowsDotPrefixAndRejectsTraversal(t *testing.T) {
	got, err := safeModelArchiveName("./" + testArchiveRoot + "/encoder.int8.onnx")
	if err != nil || got != testArchiveRoot+"/encoder.int8.onnx" {
		t.Fatalf("safeModelArchiveName(./prefix) = %q, %v", got, err)
	}
	if _, err := safeModelArchiveName(testArchiveRoot + "/../outside"); !errors.Is(err, ErrUnsafeModelArchive) {
		t.Fatalf("safeModelArchiveName(traversal) error = %v, want ErrUnsafeModelArchive", err)
	}
}

func TestModelManagerDoesNotEnableUnsupportedRuntime(t *testing.T) {
	contents := testModelContents()
	fixture := newModelTestFixture(t, makeModelArchive(t, contents), manifestForTestFiles(contents), contents, nil, false, nil)
	status := fixture.manager.Status()
	if status.Available || status.Installed || status.Error == "" {
		t.Fatalf("unsupported runtime status = %+v", status)
	}
	if err := fixture.manager.Download(context.Background()); !errors.Is(err, ErrLocalRuntimeUnavailable) {
		t.Fatalf("Download() error = %v, want ErrLocalRuntimeUnavailable", err)
	}
	if _, err := fixture.manager.LocalTranscriber(); !errors.Is(err, ErrLocalRuntimeUnavailable) {
		t.Fatalf("LocalTranscriber() error = %v, want ErrLocalRuntimeUnavailable", err)
	}
	if got := fixture.requests.Load(); got != 0 {
		t.Fatalf("unsupported runtime made %d requests, want none", got)
	}
}

func TestModelManagerDefersFileHashVerificationUntilFirstUse(t *testing.T) {
	contents := testModelContents()
	modelsDir := filepath.Join(t.TempDir(), "models")
	modelDir := filepath.Join(modelsDir, localModelDirectoryName)
	if err := os.MkdirAll(modelDir, 0o700); err != nil {
		t.Fatalf("create model directory: %v", err)
	}
	corrupt := append([]byte(nil), contents["encoder.int8.onnx"]...)
	corrupt[0] ^= 0xff
	for name, content := range contents {
		if name == "encoder.int8.onnx" {
			content = corrupt
		}
		if err := os.WriteFile(filepath.Join(modelDir, name), content, 0o600); err != nil {
			t.Fatalf("write model file %s: %v", name, err)
		}
	}
	var factoryCalls atomic.Int32
	manager := newModelManager(modelManagerOptions{
		modelsDir:        modelsDir,
		manifest:         manifestForTestFiles(contents),
		runtimeAvailable: true,
		recognizerFactory: func(modelPaths) (localRecognizer, error) {
			factoryCalls.Add(1)
			return &testLocalRecognizer{text: "unused"}, nil
		},
	})
	status := manager.Status()
	if !status.Installed || status.Verified {
		t.Fatalf("startup status = %+v, want present but not yet verified", status)
	}
	if got := factoryCalls.Load(); got != 0 {
		t.Fatalf("startup loaded recognizer %d times, want lazy loading", got)
	}
	transcriber, err := manager.LocalTranscriber()
	if err != nil {
		t.Fatalf("LocalTranscriber() before lazy verification: %v", err)
	}
	if _, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2})); !errors.Is(err, ErrLocalModelIntegrity) {
		t.Fatalf("Transcribe() error = %v, want ErrLocalModelIntegrity", err)
	}
	status = manager.Status()
	if status.Installed || status.Verified || status.Error == "" {
		t.Fatalf("status after failed verification = %+v", status)
	}
	if got := factoryCalls.Load(); got != 0 {
		t.Fatalf("recognizer factory called %d times for invalid files, want 0", got)
	}
}

func TestModelManagerDeleteWaitsForInference(t *testing.T) {
	contents := testModelContents()
	recognizer := &testLocalRecognizer{text: "done", started: make(chan struct{}), release: make(chan struct{})}
	fixture := newModelTestFixture(t, makeModelArchive(t, contents), manifestForTestFiles(contents), contents, func(modelPaths) (localRecognizer, error) {
		return recognizer, nil
	}, true, nil)
	if err := fixture.manager.Download(context.Background()); err != nil {
		t.Fatalf("Download(): %v", err)
	}
	transcriber, err := fixture.manager.LocalTranscriber()
	if err != nil {
		t.Fatalf("LocalTranscriber(): %v", err)
	}
	transcribeDone := make(chan error, 1)
	go func() {
		_, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
		transcribeDone <- err
	}()
	select {
	case <-recognizer.started:
	case <-time.After(time.Second):
		t.Fatal("local inference did not start")
	}
	deleteDone := make(chan error, 1)
	go func() { deleteDone <- fixture.manager.Delete(context.Background()) }()
	waitForModelStatus(t, fixture.manager, func(s ModelStatus) bool { return s.Deleting })
	if !fixture.manager.Status().Installed {
		t.Fatal("model was marked deleted while inference was active")
	}
	if _, err := os.Stat(filepath.Join(fixture.modelsDir, localModelDirectoryName)); err != nil {
		t.Fatalf("model files removed during inference: %v", err)
	}
	close(recognizer.release)
	if err := <-transcribeDone; err != nil {
		t.Fatalf("local transcription: %v", err)
	}
	if err := <-deleteDone; err != nil {
		t.Fatalf("Delete(): %v", err)
	}
	status := fixture.manager.Status()
	if status.Installed || status.DownloadedBytes != 0 || status.Progress != 0 {
		t.Fatalf("status after Delete() = %+v, want unavailable model and reset progress", status)
	}
}

func TestLocalTranscriberCancelsWhileWaitingForInference(t *testing.T) {
	contents := testModelContents()
	recognizer := &testLocalRecognizer{text: "done", started: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(recognizer.release) })
	fixture := newModelTestFixture(t, makeModelArchive(t, contents), manifestForTestFiles(contents), contents, func(modelPaths) (localRecognizer, error) {
		return recognizer, nil
	}, true, nil)
	if err := fixture.manager.Download(context.Background()); err != nil {
		t.Fatalf("Download(): %v", err)
	}
	transcriber, err := fixture.manager.LocalTranscriber()
	if err != nil {
		t.Fatalf("LocalTranscriber(): %v", err)
	}
	firstDone := make(chan error, 1)
	go func() {
		_, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2}))
		firstDone <- err
	}()
	select {
	case <-recognizer.started:
	case <-time.After(time.Second):
		t.Fatal("first inference did not start")
	}
	ctx, cancel := context.WithCancel(context.Background())
	secondStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		_, err := transcriber.Transcribe(ctx, makePCM16WAV(1, 16000, 16, []byte{1, 2}))
		secondDone <- err
	}()
	<-secondStarted
	cancel()
	select {
	case err := <-secondDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting inference error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued inference did not return after cancellation")
	}
	releaseOnce.Do(func() { close(recognizer.release) })
	if err := <-firstDone; err != nil {
		t.Fatalf("first inference: %v", err)
	}
}

func TestModelManagerFailedDeleteDoesNotReuseClosedRecognizer(t *testing.T) {
	contents := testModelContents()
	var recognizer *testLocalRecognizer
	fixture := newModelTestFixture(t, makeModelArchive(t, contents), manifestForTestFiles(contents), contents, func(modelPaths) (localRecognizer, error) {
		recognizer = &testLocalRecognizer{text: "loaded"}
		return recognizer, nil
	}, true, nil)
	if err := fixture.manager.Download(context.Background()); err != nil {
		t.Fatalf("Download(): %v", err)
	}
	transcriber, err := fixture.manager.LocalTranscriber()
	if err != nil {
		t.Fatalf("LocalTranscriber(): %v", err)
	}
	if _, err := transcriber.Transcribe(context.Background(), makePCM16WAV(1, 16000, 16, []byte{1, 2})); err != nil {
		t.Fatalf("load recognizer: %v", err)
	}
	target := filepath.Join(fixture.modelsDir, localModelDirectoryName)
	backup := target + ".moved"
	if err := os.Rename(target, backup); err != nil {
		t.Fatalf("move model directory: %v", err)
	}
	if err := os.Symlink(backup, target); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := fixture.manager.Delete(context.Background()); err == nil {
		t.Fatal("Delete() succeeded with a symlink model path")
	}
	status := fixture.manager.Status()
	if status.Installed {
		t.Fatalf("failed delete still reports installed: %+v", status)
	}
	if fixture.manager.recognizer != nil || recognizer.closed.Load() != 1 {
		t.Fatalf("failed delete retained a closed recognizer: current=%v close count=%d", fixture.manager.recognizer, recognizer.closed.Load())
	}
	if _, err := fixture.manager.LocalTranscriber(); !errors.Is(err, ErrLocalModelNotInstalled) {
		t.Fatalf("LocalTranscriber() after failed delete error = %v, want ErrLocalModelNotInstalled", err)
	}
}

func waitForModelStatus(t *testing.T, manager *ModelManager, predicate func(ModelStatus) bool) {
	t.Helper()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for {
		if predicate(manager.Status()) {
			return
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatal("timed out waiting for model state")
		}
	}
}

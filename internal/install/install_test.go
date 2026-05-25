package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---- helpers ---------------------------------------------------------------

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{HomeDir: t.TempDir()}
}

// writeBinary creates a minimal executable at path, creating parent dirs.
func writeBinary(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho hello"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func makeZipArchive(t *testing.T, filename, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(filename)
	if err != nil {
		t.Fatalf("zip.Create: %v", err)
	}
	if _, err := io.WriteString(f, content); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}
	return buf.Bytes()
}

func makeTarGzArchive(t *testing.T, filename, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	body := []byte(content)
	if err := tw.WriteHeader(&tar.Header{Name: filename, Size: int64(len(body)), Mode: 0o755}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	tw.Close() //nolint:errcheck // best-effort close of tar writer in test helper
	gw.Close() //nolint:errcheck // best-effort close of gzip writer in test helper
	return buf.Bytes()
}

// ---- path computation ------------------------------------------------------

func TestManagerPaths(t *testing.T) {
	mgr := &Manager{HomeDir: "/home/user/.hvm"}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"VersionDir", mgr.VersionDir("terraform", "1.9.8"), "/home/user/.hvm/versions/terraform/1.9.8"},
		{"BinDir", mgr.BinDir(), "/home/user/.hvm/bin"},
		{"BinaryPath linux", mgr.BinaryPath("terraform", "1.9.8", "linux"), "/home/user/.hvm/versions/terraform/1.9.8/terraform"},
		{"BinaryPath windows", mgr.BinaryPath("terraform", "1.9.8", "windows"), "/home/user/.hvm/versions/terraform/1.9.8/terraform.exe"},
		{"LinkPath linux", mgr.LinkPath("terraform", "linux"), "/home/user/.hvm/bin/terraform"},
		{"LinkPath windows", mgr.LinkPath("terraform", "windows"), "/home/user/.hvm/bin/terraform.exe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestHvmHomeEnvVar(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HVM_HOME", tmp)
	mgr, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if mgr.HomeDir != tmp {
		t.Errorf("HomeDir = %q, want %q", mgr.HomeDir, tmp)
	}
}

// ---- IsInstalled -----------------------------------------------------------

func TestIsInstalled(t *testing.T) {
	mgr := newTestManager(t)
	if mgr.IsInstalled("terraform", "1.0.0", runtime.GOOS) {
		t.Error("expected not installed before binary exists")
	}
	writeBinary(t, mgr.BinaryPath("terraform", "1.0.0", runtime.GOOS))
	if !mgr.IsInstalled("terraform", "1.0.0", runtime.GOOS) {
		t.Error("expected installed after binary is written")
	}
}

// ---- InstalledVersions -----------------------------------------------------

func TestInstalledVersionsEmpty(t *testing.T) {
	mgr := newTestManager(t)
	versions, err := mgr.InstalledVersions("terraform")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if versions != nil {
		t.Errorf("expected nil for no installed versions, got %v", versions)
	}
}

func TestInstalledVersionsSemverSort(t *testing.T) {
	mgr := newTestManager(t)
	// Create dirs in arbitrary order to verify sort is semver, not lexicographic.
	// Lexicographic sort would give: 1.10.0, 1.9.10, 1.9.8, 1.9.9, 2.0.0
	// Semver sort should give:       2.0.0, 1.10.0, 1.9.10, 1.9.9, 1.9.8
	for _, v := range []string{"1.9.8", "2.0.0", "1.9.10", "1.10.0", "1.9.9"} {
		if err := os.MkdirAll(mgr.VersionDir("terraform", v), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	versions, err := mgr.InstalledVersions("terraform")
	if err != nil {
		t.Fatalf("InstalledVersions: %v", err)
	}
	want := []string{"2.0.0", "1.10.0", "1.9.10", "1.9.9", "1.9.8"}
	if len(versions) != len(want) {
		t.Fatalf("versions = %v, want %v", versions, want)
	}
	for i, v := range versions {
		if v != want[i] {
			t.Errorf("versions[%d] = %q, want %q", i, v, want[i])
		}
	}
}

// ---- InstalledApps ---------------------------------------------------------

func TestInstalledAppsEmpty(t *testing.T) {
	mgr := newTestManager(t)
	apps, err := mgr.InstalledApps()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apps != nil {
		t.Errorf("expected nil for no installed apps, got %v", apps)
	}
}

func TestInstalledAppsAlphabetical(t *testing.T) {
	mgr := newTestManager(t)
	for _, app := range []string{"vault", "consul", "terraform"} {
		if err := os.MkdirAll(filepath.Join(mgr.HomeDir, "versions", app), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	apps, err := mgr.InstalledApps()
	if err != nil {
		t.Fatalf("InstalledApps: %v", err)
	}
	want := []string{"consul", "terraform", "vault"}
	if len(apps) != len(want) {
		t.Fatalf("apps = %v, want %v", apps, want)
	}
	for i, a := range apps {
		if a != want[i] {
			t.Errorf("apps[%d] = %q, want %q", i, a, want[i])
		}
	}
}

// ---- CurrentVersion --------------------------------------------------------

func TestCurrentVersionNone(t *testing.T) {
	mgr := newTestManager(t)
	ver, err := mgr.CurrentVersion("terraform", runtime.GOOS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "" {
		t.Errorf("expected empty, got %q", ver)
	}
}

func TestCurrentVersionAfterUse(t *testing.T) {
	mgr := newTestManager(t)
	writeBinary(t, mgr.BinaryPath("terraform", "1.9.8", runtime.GOOS))
	if err := mgr.Use("terraform", "1.9.8", runtime.GOOS); err != nil {
		t.Fatalf("Use: %v", err)
	}
	ver, err := mgr.CurrentVersion("terraform", runtime.GOOS)
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if ver != "1.9.8" {
		t.Errorf("CurrentVersion = %q, want %q", ver, "1.9.8")
	}
}

func TestCurrentVersionUpdatesOnSwitch(t *testing.T) {
	mgr := newTestManager(t)
	for _, v := range []string{"1.9.8", "1.9.9"} {
		writeBinary(t, mgr.BinaryPath("terraform", v, runtime.GOOS))
	}
	if err := mgr.Use("terraform", "1.9.8", runtime.GOOS); err != nil {
		t.Fatalf("Use 1.9.8: %v", err)
	}
	if err := mgr.Use("terraform", "1.9.9", runtime.GOOS); err != nil {
		t.Fatalf("Use 1.9.9: %v", err)
	}
	ver, err := mgr.CurrentVersion("terraform", runtime.GOOS)
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if ver != "1.9.9" {
		t.Errorf("CurrentVersion = %q, want %q", ver, "1.9.9")
	}
}

// ---- Use -------------------------------------------------------------------

func TestUseNotInstalled(t *testing.T) {
	mgr := newTestManager(t)
	if err := mgr.Use("terraform", "1.0.0", runtime.GOOS); err == nil {
		t.Error("expected error for non-existent binary")
	}
}

func TestUseCreatesSymlink(t *testing.T) {
	mgr := newTestManager(t)
	binPath := mgr.BinaryPath("terraform", "1.9.8", runtime.GOOS)
	writeBinary(t, binPath)

	if err := mgr.Use("terraform", "1.9.8", runtime.GOOS); err != nil {
		t.Fatalf("Use: %v", err)
	}

	link := mgr.LinkPath("terraform", runtime.GOOS)
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != binPath {
		t.Errorf("symlink target = %q, want %q", target, binPath)
	}
}

// ---- Remove ----------------------------------------------------------------

func TestRemoveNotInstalled(t *testing.T) {
	mgr := newTestManager(t)
	if err := mgr.Remove("terraform", "1.0.0", runtime.GOOS); err == nil {
		t.Error("expected error for non-existent version")
	}
}

func TestRemoveActiveVersion(t *testing.T) {
	mgr := newTestManager(t)
	writeBinary(t, mgr.BinaryPath("terraform", "1.9.8", runtime.GOOS))
	if err := mgr.Use("terraform", "1.9.8", runtime.GOOS); err != nil {
		t.Fatalf("Use: %v", err)
	}

	if err := mgr.Remove("terraform", "1.9.8", runtime.GOOS); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(mgr.VersionDir("terraform", "1.9.8")); !os.IsNotExist(err) {
		t.Error("expected version dir to be deleted")
	}
	if _, err := os.Lstat(mgr.LinkPath("terraform", runtime.GOOS)); !os.IsNotExist(err) {
		t.Error("expected symlink to be removed along with active version")
	}
}

func TestRemoveInactiveVersion(t *testing.T) {
	mgr := newTestManager(t)
	for _, v := range []string{"1.9.8", "1.9.9"} {
		writeBinary(t, mgr.BinaryPath("terraform", v, runtime.GOOS))
	}
	if err := mgr.Use("terraform", "1.9.9", runtime.GOOS); err != nil {
		t.Fatalf("Use: %v", err)
	}

	if err := mgr.Remove("terraform", "1.9.8", runtime.GOOS); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := os.Stat(mgr.VersionDir("terraform", "1.9.8")); !os.IsNotExist(err) {
		t.Error("expected version dir to be deleted")
	}
	// Symlink for the still-active version should be untouched.
	if _, err := os.Lstat(mgr.LinkPath("terraform", runtime.GOOS)); err != nil {
		t.Error("expected symlink for active version to remain")
	}
}

// ---- Download --------------------------------------------------------------

func TestDownloadZip(t *testing.T) {
	mgr := newTestManager(t)
	data := makeZipArchive(t, "terraform", "#!/bin/sh\necho hello")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(data) //nolint:errcheck // best-effort write of test response body
	}))
	defer srv.Close()

	if err := mgr.Download("terraform", "1.9.8", "linux", srv.URL+"/terraform_1.9.8_linux_amd64.zip"); err != nil {
		t.Fatalf("Download: %v", err)
	}

	info, err := os.Stat(mgr.BinaryPath("terraform", "1.9.8", "linux"))
	if err != nil {
		t.Fatalf("binary not found: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("expected binary to be executable")
	}
}

func TestDownloadTarGz(t *testing.T) {
	mgr := newTestManager(t)
	data := makeTarGzArchive(t, "terraform", "#!/bin/sh\necho hello")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(data) //nolint:errcheck // best-effort write of test response body
	}))
	defer srv.Close()

	if err := mgr.Download("terraform", "1.9.8", "linux", srv.URL+"/terraform_1.9.8_linux_amd64.tar.gz"); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if _, err := os.Stat(mgr.BinaryPath("terraform", "1.9.8", "linux")); err != nil {
		t.Fatalf("binary not found: %v", err)
	}
}

func TestDownloadHTTPError(t *testing.T) {
	mgr := newTestManager(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	if err := mgr.Download("terraform", "1.9.8", "linux", srv.URL+"/terraform_1.9.8_linux_amd64.zip"); err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestDownloadUnsupportedFormat(t *testing.T) {
	mgr := newTestManager(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("data")) //nolint:errcheck // best-effort write of test response body
	}))
	defer srv.Close()

	if err := mgr.Download("terraform", "1.9.8", "linux", srv.URL+"/terraform_1.9.8_linux_amd64.tar.bz2"); err == nil {
		t.Error("expected error for unsupported archive format")
	}
}

// Package install manages downloading, activating, and removing HashiCorp tool versions.
package install

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	goversion "github.com/hashicorp/go-version"
)

// Manager handles the local install directory.
//
// Directory layout:
//
//	~/.hvm/
//	├── bin/                 symlinks — add to PATH
//	└── versions/
//	    └── <app>/
//	        └── <version>/
//	            └── <binary>
type Manager struct {
	HomeDir string
}

// New returns a Manager rooted at HVM_HOME or ~/.hvm.
func New() (*Manager, error) {
	dir, err := homeDir()
	if err != nil {
		return nil, err
	}
	return &Manager{HomeDir: dir}, nil
}

func homeDir() (string, error) {
	if h := os.Getenv("HVM_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".hvm"), nil
}

// VersionDir returns the directory for a specific app version.
func (m *Manager) VersionDir(app, version string) string {
	return filepath.Join(m.HomeDir, "versions", app, version)
}

// BinDir returns the directory where active symlinks are stored.
func (m *Manager) BinDir() string {
	return filepath.Join(m.HomeDir, "bin")
}

// BinaryPath returns the expected path of the installed binary for app@version.
func (m *Manager) BinaryPath(app, version, targetOS string) string {
	return filepath.Join(m.VersionDir(app, version), binaryName(app, targetOS))
}

// LinkPath returns the symlink path for an app in bin/.
func (m *Manager) LinkPath(app, targetOS string) string {
	return filepath.Join(m.BinDir(), binaryName(app, targetOS))
}

// IsInstalled reports whether the binary for app@version exists on disk.
func (m *Manager) IsInstalled(app, version, targetOS string) bool {
	_, err := os.Stat(m.BinaryPath(app, version, targetOS))
	return err == nil
}

// CurrentVersion returns the version currently symlinked in bin/, or "" if none.
func (m *Manager) CurrentVersion(app, targetOS string) (string, error) {
	link := m.LinkPath(app, targetOS)
	target, err := os.Readlink(link)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading symlink %s: %w", link, err)
	}
	versionsBase := filepath.Join(m.HomeDir, "versions", app) + string(filepath.Separator)
	if !strings.HasPrefix(target, versionsBase) {
		return "", nil
	}
	rest := strings.TrimPrefix(target, versionsBase)
	parts := strings.SplitN(rest, string(filepath.Separator), 2)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], nil
}

// InstalledVersions returns all downloaded versions for app, sorted newest first.
func (m *Manager) InstalledVersions(app string) ([]string, error) {
	dir := filepath.Join(m.HomeDir, "versions", app)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading versions directory: %w", err)
	}

	type entry struct {
		raw    string
		parsed *goversion.Version
	}
	all := make([]entry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			pv, _ := goversion.NewVersion(e.Name()) //nolint:errcheck // non-semver version strings are valid; parsed value is nil and handled below
			all = append(all, entry{raw: e.Name(), parsed: pv})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		vi, vj := all[i].parsed, all[j].parsed
		if vi == nil || vj == nil {
			return all[i].raw > all[j].raw
		}
		return vi.GreaterThan(vj)
	})

	versions := make([]string, len(all))
	for i, e := range all {
		versions[i] = e.raw
	}
	return versions, nil
}

// InstalledApps returns all apps with at least one downloaded version, sorted alphabetically.
func (m *Manager) InstalledApps() ([]string, error) {
	dir := filepath.Join(m.HomeDir, "versions")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading versions directory: %w", err)
	}
	apps := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			apps = append(apps, e.Name())
		}
	}
	sort.Strings(apps)
	return apps, nil
}

// Download fetches the archive from buildURL, extracts the binary, and places
// it in the version directory.
func (m *Manager) Download(app, version, targetOS, buildURL string) error {
	destDir := m.VersionDir(app, version)
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("creating version directory: %w", err)
	}

	// #nosec G107 -- URL sourced from the HashiCorp releases API
	resp, err := http.Get(buildURL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", buildURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable after successful read

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d downloading %s", resp.StatusCode, buildURL)
	}

	tmp, err := os.CreateTemp("", "hvm-download-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup of temp file; failure is non-critical

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close() //nolint:errcheck // best-effort close before returning the copy error
		return fmt.Errorf("writing download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	binName := binaryName(app, targetOS)
	destPath := filepath.Join(destDir, binName)

	switch {
	case strings.HasSuffix(buildURL, ".zip"):
		return extractZip(tmpName, binName, destPath)
	case strings.HasSuffix(buildURL, ".tar.gz"):
		return extractTarGz(tmpName, binName, destPath)
	default:
		return fmt.Errorf("unsupported archive format: %s", buildURL)
	}
}

// Use creates or replaces the symlink in bin/ pointing app to the given version.
func (m *Manager) Use(app, version, targetOS string) error {
	binPath := m.BinaryPath(app, version, targetOS)
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return fmt.Errorf("%s@%s is not installed", app, version)
	}
	if err := os.MkdirAll(m.BinDir(), 0o750); err != nil {
		return fmt.Errorf("creating bin directory: %w", err)
	}
	link := m.LinkPath(app, targetOS)
	_ = os.Remove(link) //nolint:errcheck // best-effort removal of old symlink; may not exist yet
	if err := os.Symlink(binPath, link); err != nil {
		return fmt.Errorf("creating symlink %s -> %s: %w", link, binPath, err)
	}
	return nil
}

// Remove deletes the downloaded version directory. If the version is currently
// active the symlink in bin/ is also removed.
func (m *Manager) Remove(app, version, targetOS string) error {
	dir := m.VersionDir(app, version)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("%s@%s is not installed", app, version)
	}
	cur, _ := m.CurrentVersion(app, targetOS) //nolint:errcheck // symlink may not exist; non-existence is handled as empty string
	if cur == version {
		_ = os.Remove(m.LinkPath(app, targetOS)) //nolint:errcheck // best-effort removal of active symlink before deleting version directory
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing %s@%s: %w", app, version, err)
	}
	return nil
}

func binaryName(app, targetOS string) string {
	if targetOS == "windows" {
		return app + ".exe"
	}
	return app
}

func extractZip(archivePath, binName, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close() //nolint:errcheck // best-effort close of zip reader

	for _, f := range r.File {
		if filepath.Base(f.Name) != binName {
			continue
		}
		if err := extractZipEntry(f, destPath); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("binary %q not found in archive", binName)
}

func extractZipEntry(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening %s in zip: %w", f.Name, err)
	}
	defer rc.Close() //nolint:errcheck // best-effort close of zip entry reader
	return writeExecutable(rc, destPath)
}

func extractTarGz(archivePath, binName, destPath string) error {
	f, err := os.Open(archivePath) // #nosec G304 -- archivePath is a temp file created by this process
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer f.Close() //nolint:errcheck // best-effort close of archive file

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading gzip: %w", err)
	}
	defer gz.Close() //nolint:errcheck // best-effort close of gzip reader

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}
		if filepath.Base(hdr.Name) != binName {
			continue
		}
		return writeExecutable(tr, destPath)
	}
	return fmt.Errorf("binary %q not found in archive", binName)
}

func writeExecutable(r io.Reader, destPath string) error {
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) // #nosec G302 G304 -- 0755 intentional for executable binary; path is constructed from trusted install dir
	if err != nil {
		return fmt.Errorf("creating binary at %s: %w", destPath, err)
	}
	defer f.Close() //nolint:errcheck // best-effort close; write errors are caught by io.Copy
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("writing binary: %w", err)
	}
	return nil
}

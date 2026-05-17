// Package hvmrc reads .hvmrc files to resolve pinned tool versions.
//
// Format: one "app=version" entry per line, # comments supported.
//
//	# .hvmrc
//	terraform=1.9.8
//	vault=1.15.3
package hvmrc

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Lookup searches for an .hvmrc file starting at dir and walking toward the
// filesystem root. Returns the pinned version and the file path where it was
// found, or ("", "") if no entry exists for app.
func Lookup(app, dir string) (version, file string) {
	for {
		path := filepath.Join(dir, ".hvmrc")
		if v := readVersion(path, app); v != "" {
			return v, path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", ""
}

// LookupAll searches for an .hvmrc file starting at dir and walking toward the
// filesystem root. Returns all app=version entries found, plus the file path,
// or (nil, "") if no .hvmrc file is found.
func LookupAll(dir string) (entries map[string]string, file string) {
	for {
		path := filepath.Join(dir, ".hvmrc")
		if m := readAll(path); m != nil {
			return m, path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, ""
}

func readAll(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close() //nolint:errcheck

	m := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if k, v := strings.TrimSpace(key), strings.TrimSpace(val); k != "" && v != "" {
			m[k] = v
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func readVersion(path, app string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) == app {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

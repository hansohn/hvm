package hvmrc

import (
	"os"
	"path/filepath"
	"testing"
)

func writeHvmrc(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, ".hvmrc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write .hvmrc: %v", err)
	}
	return path
}

// ---- Lookup ----------------------------------------------------------------

func TestLookupCurrentDir(t *testing.T) {
	dir := t.TempDir()
	writeHvmrc(t, dir, "terraform=1.9.8\nvault=1.15.3\n")

	ver, file := Lookup("terraform", dir)
	if ver != "1.9.8" {
		t.Errorf("version = %q, want %q", ver, "1.9.8")
	}
	if file != filepath.Join(dir, ".hvmrc") {
		t.Errorf("file = %q, want %q", file, filepath.Join(dir, ".hvmrc"))
	}
}

func TestLookupSecondEntry(t *testing.T) {
	dir := t.TempDir()
	writeHvmrc(t, dir, "terraform=1.9.8\nvault=1.15.3\n")

	ver, _ := Lookup("vault", dir)
	if ver != "1.15.3" {
		t.Errorf("version = %q, want %q", ver, "1.15.3")
	}
}

func TestLookupWalksUp(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "sub", "project")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	writeHvmrc(t, parent, "terraform=1.9.8\n")

	ver, file := Lookup("terraform", child)
	if ver != "1.9.8" {
		t.Errorf("version = %q, want %q", ver, "1.9.8")
	}
	if file != filepath.Join(parent, ".hvmrc") {
		t.Errorf("file = %q, want %q", file, filepath.Join(parent, ".hvmrc"))
	}
}

func TestLookupChildOverridesParent(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "project")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	writeHvmrc(t, parent, "terraform=1.9.8\n")
	writeHvmrc(t, child, "terraform=1.10.0\n")

	ver, _ := Lookup("terraform", child)
	if ver != "1.10.0" {
		t.Errorf("version = %q, want %q (child should override parent)", ver, "1.10.0")
	}
}

func TestLookupNotFound(t *testing.T) {
	dir := t.TempDir()
	ver, file := Lookup("terraform", dir)
	if ver != "" || file != "" {
		t.Errorf("expected empty results, got (%q, %q)", ver, file)
	}
}

func TestLookupAppNotInFile(t *testing.T) {
	dir := t.TempDir()
	writeHvmrc(t, dir, "vault=1.15.3\n")

	ver, file := Lookup("terraform", dir)
	if ver != "" || file != "" {
		t.Errorf("expected empty results, got (%q, %q)", ver, file)
	}
}

func TestLookupSkipsComments(t *testing.T) {
	dir := t.TempDir()
	writeHvmrc(t, dir, "# pinned versions\n\nterraform=1.9.8\n")

	ver, _ := Lookup("terraform", dir)
	if ver != "1.9.8" {
		t.Errorf("version = %q, want %q", ver, "1.9.8")
	}
}

func TestLookupTrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	writeHvmrc(t, dir, "  terraform = 1.9.8  \n")

	ver, _ := Lookup("terraform", dir)
	if ver != "1.9.8" {
		t.Errorf("version = %q, want %q", ver, "1.9.8")
	}
}

// ---- LookupAll -------------------------------------------------------------

func TestLookupAllBasic(t *testing.T) {
	dir := t.TempDir()
	writeHvmrc(t, dir, "terraform=1.9.8\nvault=1.15.3\n")

	entries, file := LookupAll(dir)
	if file != filepath.Join(dir, ".hvmrc") {
		t.Errorf("file = %q, want %q", file, filepath.Join(dir, ".hvmrc"))
	}
	if entries["terraform"] != "1.9.8" {
		t.Errorf("terraform = %q, want %q", entries["terraform"], "1.9.8")
	}
	if entries["vault"] != "1.15.3" {
		t.Errorf("vault = %q, want %q", entries["vault"], "1.15.3")
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestLookupAllNotFound(t *testing.T) {
	dir := t.TempDir()
	entries, file := LookupAll(dir)
	if entries != nil || file != "" {
		t.Errorf("expected nil results, got (%v, %q)", entries, file)
	}
}

func TestLookupAllCommentOnlyFile(t *testing.T) {
	dir := t.TempDir()
	writeHvmrc(t, dir, "# only comments\n\n")

	entries, file := LookupAll(dir)
	if entries != nil || file != "" {
		t.Errorf("expected nil results for comment-only file, got (%v, %q)", entries, file)
	}
}

func TestLookupAllSkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	writeHvmrc(t, dir, "terraform=1.9.8\nno-equals-sign\nvault=1.15.3\n")

	entries, _ := LookupAll(dir)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (malformed line skipped), got %d: %v", len(entries), entries)
	}
	if entries["terraform"] != "1.9.8" {
		t.Errorf("terraform = %q, want %q", entries["terraform"], "1.9.8")
	}
	if entries["vault"] != "1.15.3" {
		t.Errorf("vault = %q, want %q", entries["vault"], "1.15.3")
	}
}

func TestLookupAllWalksUp(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "project")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	writeHvmrc(t, parent, "terraform=1.9.8\nvault=1.15.3\n")

	entries, file := LookupAll(child)
	if file != filepath.Join(parent, ".hvmrc") {
		t.Errorf("file = %q, want %q", file, filepath.Join(parent, ".hvmrc"))
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

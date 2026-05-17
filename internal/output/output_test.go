package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hansohn/hvm/internal/releases"
)

// captureStdout redirects os.Stdout for the duration of fn and returns whatever was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}

// ---- format routing (error / no-error) ------------------------------------

func TestWriteFormatRouting(t *testing.T) {
	tests := []struct {
		name        string
		data        any
		format      string
		expectError bool
	}{
		{name: "json format", data: []string{"v1", "v2"}, format: "json"},
		{name: "yaml format", data: []string{"v1", "v2"}, format: "yaml"},
		{name: "text format", data: []string{"v1", "v2"}, format: "text"},
		{name: "empty format defaults to text", data: []string{"v1"}, format: ""},
		{name: "unsupported format", data: []string{"v1"}, format: "xml", expectError: true},
		{name: "unsupported type for text", data: 42, format: "text", expectError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureStdout(t, func() {
				err := Write(tt.data, tt.format)
				if (err != nil) != tt.expectError {
					t.Errorf("Write() error = %v, expectError %v", err, tt.expectError)
				}
			})
		})
	}
}

// ---- JSON content ----------------------------------------------------------

func TestWriteJSONStringSlice(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Write([]string{"terraform", "vault"}, "json"); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	want := []string{"terraform", "vault"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWriteJSONVersionOutput(t *testing.T) {
	vo := VersionOutput{
		Application: "terraform",
		Version:     "1.9.8",
		URL:         "https://releases.hashicorp.com/terraform/1.9.8/",
	}
	out := captureStdout(t, func() {
		if err := Write(vo, "json"); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if got["application"] != "terraform" {
		t.Errorf("application = %v, want %q", got["application"], "terraform")
	}
	if got["version"] != "1.9.8" {
		t.Errorf("version = %v, want %q", got["version"], "1.9.8")
	}
}

// ---- YAML content ----------------------------------------------------------

func TestWriteYAMLStringSlice(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Write([]string{"terraform", "vault"}, "yaml"); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	if !strings.Contains(out, "terraform") || !strings.Contains(out, "vault") {
		t.Errorf("YAML output missing expected values: %s", out)
	}
}

// ---- text content ----------------------------------------------------------

func TestWriteTextStringSlice(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Write([]string{"terraform", "vault", "consul"}, "text"); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	for _, want := range []string{"terraform", "vault", "consul"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q: %s", want, out)
		}
	}
}

func TestWriteTextVersionOutput(t *testing.T) {
	vo := VersionOutput{
		Application: "terraform",
		Version:     "1.9.8",
		URL:         "https://releases.hashicorp.com/terraform/1.9.8/",
		Platform:    &PlatformInfo{OS: "linux", Arch: "amd64"},
		Metadata: &releases.VersionMetadata{
			Builds: []releases.Build{{OS: "linux", Arch: "amd64", URL: "https://example.com/terraform.zip"}},
			Files:  []string{"terraform_1.9.8_linux_amd64.zip"},
		},
	}
	out := captureStdout(t, func() {
		if err := Write(vo, "text"); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	for _, want := range []string{"terraform", "1.9.8", "linux", "amd64"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
}

func TestWriteTextVersionOutputSlice(t *testing.T) {
	vos := []VersionOutput{
		{Application: "terraform", Version: "1.9.8", URL: "https://example.com/"},
		{Application: "vault", Version: "1.15.3", URL: "https://example.com/"},
	}
	out := captureStdout(t, func() {
		if err := Write(vos, "text"); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	for _, want := range []string{"terraform", "1.9.8", "vault", "1.15.3"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
}

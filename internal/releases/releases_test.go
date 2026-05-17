package releases

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// ---- FilterAndSortBuilds ---------------------------------------------------

func TestFilterAndSortBuilds(t *testing.T) {
	builds := []Build{
		{OS: "linux", Arch: "amd64", URL: "linux_amd64.zip"},
		{OS: "linux", Arch: "arm64", URL: "linux_arm64.zip"},
		{OS: "darwin", Arch: "amd64", URL: "darwin_amd64.zip"},
		{OS: "darwin", Arch: "arm64", URL: "darwin_arm64.zip"},
		{OS: "windows", Arch: "amd64", URL: "windows_amd64.zip"},
	}

	tests := []struct {
		name                string
		targetOS            string
		targetArch          string
		expectedCurrent     []Build
		expectedOthersCount int
	}{
		{
			name:                "linux amd64",
			targetOS:            "linux",
			targetArch:          "amd64",
			expectedCurrent:     []Build{{OS: "linux", Arch: "amd64", URL: "linux_amd64.zip"}},
			expectedOthersCount: 4,
		},
		{
			name:                "darwin arm64",
			targetOS:            "darwin",
			targetArch:          "arm64",
			expectedCurrent:     []Build{{OS: "darwin", Arch: "arm64", URL: "darwin_arm64.zip"}},
			expectedOthersCount: 4,
		},
		{
			name:                "no match",
			targetOS:            "freebsd",
			targetArch:          "amd64",
			expectedCurrent:     nil,
			expectedOthersCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, others := FilterAndSortBuilds(builds, tt.targetOS, tt.targetArch)
			if !reflect.DeepEqual(current, tt.expectedCurrent) {
				t.Errorf("current = %v, want %v", current, tt.expectedCurrent)
			}
			if len(others) != tt.expectedOthersCount {
				t.Errorf("others count = %d, want %d", len(others), tt.expectedOthersCount)
			}
		})
	}
}

// ---- FilterFilesByPlatform -------------------------------------------------

func TestFilterFilesByPlatform(t *testing.T) {
	files := []string{
		"terraform_1.9.8_linux_amd64.zip",
		"terraform_1.9.8_linux_arm64.zip",
		"terraform_1.9.8_darwin_amd64.zip",
		"terraform_1.9.8_darwin_arm64.zip",
		"terraform_1.9.8_windows_amd64.zip",
		"terraform_1.9.8_SHA256SUMS",
		"terraform_1.9.8_SHA256SUMS.sig",
		"terraform_1.9.8_SHA256SUMS.72D7468F.sig",
	}

	tests := []struct {
		name          string
		targetOS      string
		targetArch    string
		expectedFiles []string
	}{
		{
			name:       "linux amd64",
			targetOS:   "linux",
			targetArch: "amd64",
			expectedFiles: []string{
				"terraform_1.9.8_linux_amd64.zip",
				"terraform_1.9.8_SHA256SUMS",
				"terraform_1.9.8_SHA256SUMS.sig",
				"terraform_1.9.8_SHA256SUMS.72D7468F.sig",
			},
		},
		{
			name:       "darwin arm64",
			targetOS:   "darwin",
			targetArch: "arm64",
			expectedFiles: []string{
				"terraform_1.9.8_darwin_arm64.zip",
				"terraform_1.9.8_SHA256SUMS",
				"terraform_1.9.8_SHA256SUMS.sig",
				"terraform_1.9.8_SHA256SUMS.72D7468F.sig",
			},
		},
		{
			name:       "windows amd64",
			targetOS:   "windows",
			targetArch: "amd64",
			expectedFiles: []string{
				"terraform_1.9.8_windows_amd64.zip",
				"terraform_1.9.8_SHA256SUMS",
				"terraform_1.9.8_SHA256SUMS.sig",
				"terraform_1.9.8_SHA256SUMS.72D7468F.sig",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterFilesByPlatform(files, tt.targetOS, tt.targetArch)
			if !reflect.DeepEqual(result, tt.expectedFiles) {
				t.Errorf("FilterFilesByPlatform() = %v, want %v", result, tt.expectedFiles)
			}
		})
	}
}

// ---- FetchApplications -----------------------------------------------------

func TestFetchApplications(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
<a href="/consul/">consul</a>
<a href="/terraform/">terraform</a>
<a href="/vault/">vault</a>
<a href="/consul/">consul</a>
<a href="https://www.hashicorp.com">external link filtered by dot</a>
</body></html>`)
	}))
	defer srv.Close()

	apps, err := New(srv.URL).FetchApplications()
	if err != nil {
		t.Fatalf("FetchApplications: %v", err)
	}
	want := []string{"consul", "terraform", "vault"}
	if !reflect.DeepEqual(apps, want) {
		t.Errorf("apps = %v, want %v", apps, want)
	}
}

func TestFetchApplicationsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL).FetchApplications()
	if err == nil {
		t.Error("expected error for non-200 response")
	}
}

// ---- FetchVersions ---------------------------------------------------------

func TestFetchVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
<a href="/terraform/1.9.8/">1.9.8</a>
<a href="/terraform/1.9.9/">1.9.9</a>
<a href="/terraform/1.9.10/">1.9.10</a>
<a href="/terraform/2.0.0/">2.0.0</a>
</body></html>`)
	}))
	defer srv.Close()

	versions, err := New(srv.URL).FetchVersions("terraform")
	if err != nil {
		t.Fatalf("FetchVersions: %v", err)
	}
	// Verify semver ordering: 1.9.10 must sort above 1.9.9 (would fail under lexicographic sort).
	want := []string{"2.0.0", "1.9.10", "1.9.9", "1.9.8"}
	if !reflect.DeepEqual(versions, want) {
		t.Errorf("versions = %v, want %v", versions, want)
	}
}

func TestFetchVersionsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := New(srv.URL).FetchVersions("terraform")
	if err == nil {
		t.Error("expected error for non-200 response")
	}
}

// ---- FetchVersionMetadata --------------------------------------------------

func TestFetchVersionMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>
<a href="terraform_1.9.8_linux_amd64.zip">terraform_1.9.8_linux_amd64.zip</a>
<a href="terraform_1.9.8_darwin_arm64.zip">terraform_1.9.8_darwin_arm64.zip</a>
<a href="terraform_1.9.8_SHA256SUMS">terraform_1.9.8_SHA256SUMS</a>
<a href="terraform_1.9.8_SHA256SUMS.sig">terraform_1.9.8_SHA256SUMS.sig</a>
<a href="../">parent — must be excluded</a>
</body></html>`)
	}))
	defer srv.Close()

	meta, err := New(srv.URL).FetchVersionMetadata("terraform", "1.9.8")
	if err != nil {
		t.Fatalf("FetchVersionMetadata: %v", err)
	}

	if len(meta.Builds) != 2 {
		t.Errorf("builds = %d, want 2: %v", len(meta.Builds), meta.Builds)
	}

	var linuxBuild *Build
	for i := range meta.Builds {
		if meta.Builds[i].OS == "linux" && meta.Builds[i].Arch == "amd64" {
			linuxBuild = &meta.Builds[i]
		}
	}
	if linuxBuild == nil {
		t.Fatal("linux/amd64 build not found")
	}
	if linuxBuild.URL != "terraform_1.9.8_linux_amd64.zip" {
		t.Errorf("URL = %q, want %q", linuxBuild.URL, "terraform_1.9.8_linux_amd64.zip")
	}

	// 2 zip files + SHA256SUMS + .sig = 4 files; parent "../" excluded
	if len(meta.Files) != 4 {
		t.Errorf("files = %d, want 4: %v", len(meta.Files), meta.Files)
	}
}

func TestFetchVersionMetadataHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := New(srv.URL).FetchVersionMetadata("terraform", "1.9.8")
	if err == nil {
		t.Error("expected error for non-200 response")
	}
}

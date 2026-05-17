// Package releases fetches product release data from the HashiCorp releases API.
package releases

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/go-version"
	"golang.org/x/net/html"
)

// DefaultBaseURL is the canonical HashiCorp releases endpoint.
const DefaultBaseURL = "https://releases.hashicorp.com"

var (
	versionRegex = regexp.MustCompile(`^[\d]+\.[\d]+\.[\d]+`)
	buildRegex   = regexp.MustCompile(`([^_/]+)_([^_/]+)_([^_/]+)_([^_/.]+)\.(zip|tar\.gz)$`)
)

// Build represents a single platform artifact for a release.
type Build struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	URL  string `json:"url"`
}

// VersionMetadata holds the build artifacts and file list for a specific version.
type VersionMetadata struct {
	Builds []Build  `json:"builds"`
	Files  []string `json:"files"`
}

// Client fetches release data from the HashiCorp releases site.
type Client struct {
	BaseURL    string
	httpClient *http.Client
}

// New returns a Client using the given base URL.
func New(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{},
	}
}

// FetchApplications returns a sorted, deduplicated list of available applications.
func (c *Client) FetchApplications() ([]string, error) {
	resp, err := c.httpClient.Get(c.BaseURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("failed to fetch applications: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	appSet := make(map[string]bool)
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					app := strings.Trim(attr.Val, "/")
					if app != "" && !strings.Contains(app, ".") {
						appSet[app] = true
					}
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)

	apps := make([]string, 0, len(appSet))
	for app := range appSet {
		apps = append(apps, app)
	}
	sort.Strings(apps)

	return apps, nil
}

// FetchVersions returns versions for app sorted newest first using semantic versioning.
func (c *Client) FetchVersions(app string) ([]string, error) {
	url := fmt.Sprintf("%s/%s/", c.BaseURL, app)
	// #nosec G107 -- URL constructed from user-configured BaseURL and app name from trusted source
	resp, err := c.httpClient.Get(url) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions for %s: %w", app, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	versionSet := make(map[string]bool)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					parts := strings.Split(strings.Trim(attr.Val, "/"), "/")
					if len(parts) > 0 {
						ver := parts[len(parts)-1]
						if versionRegex.MatchString(ver) {
							versionSet[ver] = true
						}
					}
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)

	type versionEntry struct {
		raw    string
		parsed *version.Version
	}
	entries := make([]versionEntry, 0, len(versionSet))
	for v := range versionSet {
		pv, _ := version.NewVersion(v)
		entries = append(entries, versionEntry{raw: v, parsed: pv})
	}

	sort.Slice(entries, func(i, j int) bool {
		vi, vj := entries[i].parsed, entries[j].parsed
		if vi == nil || vj == nil {
			return entries[i].raw > entries[j].raw
		}
		return vi.GreaterThan(vj)
	})

	versions := make([]string, len(entries))
	for i, e := range entries {
		versions[i] = e.raw
	}
	return versions, nil
}

// FetchVersionMetadata returns build and file information for a specific app version.
func (c *Client) FetchVersionMetadata(app, ver string) (*VersionMetadata, error) {
	url := fmt.Sprintf("%s/%s/%s/", c.BaseURL, app, ver)
	// #nosec G107 -- URL constructed from user-configured BaseURL and values from trusted source
	resp, err := c.httpClient.Get(url) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata for %s %s: %w", app, ver, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	metadata := &VersionMetadata{
		Builds: make([]Build, 0),
		Files:  make([]string, 0),
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href := attr.Val
					if strings.Contains(href, "..") {
						continue
					}
					parts := strings.Split(strings.TrimRight(href, "/"), "/")
					filename := parts[len(parts)-1]
					if filename != "" && filename != app && filename != ver {
						metadata.Files = append(metadata.Files, filename)
						if matches := buildRegex.FindStringSubmatch(filename); len(matches) >= 5 {
							metadata.Builds = append(metadata.Builds, Build{
								OS:   matches[3],
								Arch: matches[4],
								URL:  href,
							})
						}
					}
				}
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)

	return metadata, nil
}

// FilterAndSortBuilds splits builds into those matching the target platform and all others.
func FilterAndSortBuilds(builds []Build, targetOS, targetArch string) (current, others []Build) {
	for _, b := range builds {
		if b.OS == targetOS && b.Arch == targetArch {
			current = append(current, b)
		} else {
			others = append(others, b)
		}
	}
	return current, others
}

// FilterFilesByPlatform returns checksum/signature files and files matching the target platform.
func FilterFilesByPlatform(files []string, targetOS, targetArch string) []string {
	platformSuffix := fmt.Sprintf("_%s_%s", targetOS, targetArch)

	var filtered []string
	for _, file := range files {
		if strings.Contains(file, "SHA256SUMS") || strings.HasSuffix(file, ".sig") || file == "terms" {
			filtered = append(filtered, file)
			continue
		}
		if strings.Contains(file, platformSuffix) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

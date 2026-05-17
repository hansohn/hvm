// Package filter provides pure functions for filtering and selecting HashiCorp release versions.
package filter

import (
	"strings"

	"github.com/hashicorp/go-version"
)

// FilterEnterpriseVersions filters versions based on enterprise and HSM flags.
//
// Filtering is scoped by the flag combination:
//   - No flags:      only community versions (no +ent)
//   - -e only:       only enterprise versions (+ent, excluding .hsm)
//   - -e --hsm:      only HSM versions (.hsm)
func FilterEnterpriseVersions(versions []string, includeEnterprise, includeHSM bool) []string {
	filtered := make([]string, 0, len(versions))
	for _, v := range versions {
		isEnterprise := strings.Contains(v, "+ent")
		isHSM := strings.Contains(v, ".hsm")

		switch {
		case includeEnterprise && includeHSM:
			if isHSM {
				filtered = append(filtered, v)
			}
		case includeEnterprise:
			if isEnterprise && !isHSM {
				filtered = append(filtered, v)
			}
		default:
			if !isEnterprise {
				filtered = append(filtered, v)
			}
		}
	}
	return filtered
}

// FilterPreReleaseVersions removes pre-release versions (alpha, beta, rc) when
// includePreRelease is false.
func FilterPreReleaseVersions(versions []string, includePreRelease bool) []string {
	if includePreRelease {
		return versions
	}

	filtered := make([]string, 0, len(versions))
	for _, v := range versions {
		ver, err := version.NewVersion(v)
		if err != nil {
			filtered = append(filtered, v)
			continue
		}
		if ver.Prerelease() == "" {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// LimitVersionCount returns at most count versions. A count of -1 returns all versions.
func LimitVersionCount(versions []string, count int) []string {
	if count == -1 || count >= len(versions) {
		return versions
	}
	if count < 0 {
		count = 0
	}
	return versions[:count]
}

// FilterVersionsByPattern returns all versions matching the given pattern.
// Versions must be pre-sorted newest first.
//
// Pattern matching:
//   - "latest" returns only the first (newest) version
//   - Exact match (e.g. "1.9.8") returns that single version
//   - Prefix match (e.g. "1.9") returns all matching versions (e.g. all 1.9.x)
func FilterVersionsByPattern(pattern string, versions []string) []string {
	if pattern == "latest" {
		if len(versions) > 0 {
			return versions[:1]
		}
		return nil
	}

	for _, v := range versions {
		if v == pattern {
			return []string{v}
		}
	}

	var matched []string
	for _, v := range versions {
		if strings.HasPrefix(v, pattern) {
			matched = append(matched, v)
		}
	}
	return matched
}

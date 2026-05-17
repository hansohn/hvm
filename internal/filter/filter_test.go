package filter

import (
	"reflect"
	"testing"
)

func TestFilterEnterpriseVersions(t *testing.T) {
	tests := []struct {
		name              string
		versions          []string
		includeEnterprise bool
		includeHSM        bool
		expectedVersions  []string
	}{
		{
			name: "community only (default)",
			versions: []string{
				"1.9.9",
				"1.9.9+ent",
				"1.9.9+ent.hsm",
				"1.9.8",
				"1.9.8+ent",
			},
			includeEnterprise: false,
			includeHSM:        false,
			expectedVersions:  []string{"1.9.9", "1.9.8"},
		},
		{
			name: "enterprise only (no HSM)",
			versions: []string{
				"1.9.9",
				"1.9.9+ent",
				"1.9.9+ent.hsm",
				"1.9.8",
				"1.9.8+ent",
			},
			includeEnterprise: true,
			includeHSM:        false,
			expectedVersions:  []string{"1.9.9+ent", "1.9.8+ent"},
		},
		{
			name: "HSM only",
			versions: []string{
				"1.9.9",
				"1.9.9+ent",
				"1.9.9+ent.hsm",
				"1.9.9+ent.hsm.fips1403",
				"1.9.8",
				"1.9.8+ent",
			},
			includeEnterprise: true,
			includeHSM:        true,
			expectedVersions:  []string{"1.9.9+ent.hsm", "1.9.9+ent.hsm.fips1403"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterEnterpriseVersions(tt.versions, tt.includeEnterprise, tt.includeHSM)
			if !reflect.DeepEqual(result, tt.expectedVersions) {
				t.Errorf("FilterEnterpriseVersions() = %v, want %v", result, tt.expectedVersions)
			}
		})
	}
}

func TestFilterPreReleaseVersions(t *testing.T) {
	tests := []struct {
		name              string
		versions          []string
		includePreRelease bool
		expectedVersions  []string
	}{
		{
			name: "exclude pre-release (default)",
			versions: []string{
				"1.14.0-rc2",
				"1.14.0-rc1",
				"1.14.0-beta1",
				"1.13.5",
				"1.13.4",
			},
			includePreRelease: false,
			expectedVersions:  []string{"1.13.5", "1.13.4"},
		},
		{
			name: "include pre-release",
			versions: []string{
				"1.14.0-rc2",
				"1.14.0-rc1",
				"1.14.0-beta1",
				"1.13.5",
				"1.13.4",
			},
			includePreRelease: true,
			expectedVersions: []string{
				"1.14.0-rc2",
				"1.14.0-rc1",
				"1.14.0-beta1",
				"1.13.5",
				"1.13.4",
			},
		},
		{
			name:              "all stable versions",
			versions:          []string{"1.13.5", "1.13.4", "1.13.3"},
			includePreRelease: false,
			expectedVersions:  []string{"1.13.5", "1.13.4", "1.13.3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterPreReleaseVersions(tt.versions, tt.includePreRelease)
			if !reflect.DeepEqual(result, tt.expectedVersions) {
				t.Errorf("FilterPreReleaseVersions() = %v, want %v", result, tt.expectedVersions)
			}
		})
	}
}

func TestLimitVersionCount(t *testing.T) {
	versions := []string{"1.9.9", "1.9.8", "1.9.7", "1.9.6", "1.9.5"}

	tests := []struct {
		name             string
		count            int
		expectedVersions []string
	}{
		{name: "limit to 3", count: 3, expectedVersions: []string{"1.9.9", "1.9.8", "1.9.7"}},
		{name: "limit to 1", count: 1, expectedVersions: []string{"1.9.9"}},
		{name: "all versions (-1)", count: -1, expectedVersions: versions},
		{name: "count larger than available", count: 10, expectedVersions: versions},
		{name: "count 0", count: 0, expectedVersions: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LimitVersionCount(versions, tt.count)
			if !reflect.DeepEqual(result, tt.expectedVersions) {
				t.Errorf("LimitVersionCount() = %v, want %v", result, tt.expectedVersions)
			}
		})
	}
}

func TestFilterVersionsByPattern(t *testing.T) {
	// Versions sorted newest first, as they would be in real usage.
	versions := []string{
		"2.0.0",
		"1.9.10",
		"1.9.9",
		"1.9.8",
		"1.8.5",
		"1.8.4",
	}

	tests := []struct {
		name     string
		pattern  string
		versions []string
		expected []string
	}{
		{
			name:     "latest keyword",
			pattern:  "latest",
			versions: versions,
			expected: []string{"2.0.0"},
		},
		{
			name:     "exact match",
			pattern:  "1.9.8",
			versions: versions,
			expected: []string{"1.9.8"},
		},
		{
			name:     "major.minor prefix returns all matching",
			pattern:  "1.9",
			versions: versions,
			expected: []string{"1.9.10", "1.9.9", "1.9.8"},
		},
		{
			name:     "major prefix returns all matching",
			pattern:  "1",
			versions: versions,
			expected: []string{"1.9.10", "1.9.9", "1.9.8", "1.8.5", "1.8.4"},
		},
		{
			name:     "no match returns empty",
			pattern:  "3.0",
			versions: versions,
			expected: nil,
		},
		{
			name:     "latest with empty list",
			pattern:  "latest",
			versions: []string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterVersionsByPattern(tt.pattern, tt.versions)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("FilterVersionsByPattern() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCombinedFilters(t *testing.T) {
	versions := []string{
		"1.14.0-rc2",
		"1.14.0-rc1",
		"1.13.5",
		"1.13.5+ent",
		"1.13.5+ent.hsm",
		"1.13.4",
		"1.13.4+ent",
		"1.13.3",
		"1.13.3+ent",
		"1.13.2-beta1",
	}

	tests := []struct {
		name              string
		includePreRelease bool
		includeEnterprise bool
		includeHSM        bool
		count             int
		expectedVersions  []string
	}{
		{
			name:             "stable community, limit 3",
			count:            3,
			expectedVersions: []string{"1.13.5", "1.13.4", "1.13.3"},
		},
		{
			name:              "stable enterprise only, limit 2",
			includeEnterprise: true,
			count:             2,
			expectedVersions:  []string{"1.13.5+ent", "1.13.4+ent"},
		},
		{
			name:              "all pre-release community, limit 5",
			includePreRelease: true,
			count:             5,
			expectedVersions:  []string{"1.14.0-rc2", "1.14.0-rc1", "1.13.5", "1.13.4", "1.13.3"},
		},
		{
			name:              "HSM only, limit 1",
			includeEnterprise: true,
			includeHSM:        true,
			count:             1,
			expectedVersions:  []string{"1.13.5+ent.hsm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterPreReleaseVersions(versions, tt.includePreRelease)
			result = FilterEnterpriseVersions(result, tt.includeEnterprise, tt.includeHSM)
			result = LimitVersionCount(result, tt.count)
			if !reflect.DeepEqual(result, tt.expectedVersions) {
				t.Errorf("combined filters = %v, want %v", result, tt.expectedVersions)
			}
		})
	}
}

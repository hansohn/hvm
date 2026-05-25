package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
)

// ---- buildAppItems ---------------------------------------------------------

func TestBuildAppItemsEmpty(t *testing.T) {
	items := buildAppItems(nil, nil)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestBuildAppItemsNoCurrentVersions(t *testing.T) {
	apps := []string{"consul", "terraform", "vault"}
	items := buildAppItems(apps, nil)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	for i, raw := range items {
		ai, ok := raw.(appItem)
		if !ok {
			t.Fatalf("items[%d] is not an appItem", i)
		}
		if ai.name != apps[i] {
			t.Errorf("items[%d].name = %q, want %q", i, ai.name, apps[i])
		}
		if ai.current != "" {
			t.Errorf("items[%d].current = %q, want empty", i, ai.current)
		}
	}
}

func TestBuildAppItemsWithCurrentVersions(t *testing.T) {
	apps := []string{"consul", "terraform", "vault"}
	currents := map[string]string{
		"terraform": "1.9.8",
		"vault":     "1.15.3",
	}
	items := buildAppItems(apps, currents)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// consul has no current version
	consul := items[0].(appItem)
	if consul.name != "consul" {
		t.Errorf("items[0].name = %q, want %q", consul.name, "consul")
	}
	if consul.current != "" {
		t.Errorf("items[0].current = %q, want empty", consul.current)
	}

	// terraform has a current version
	tf := items[1].(appItem)
	if tf.name != "terraform" {
		t.Errorf("items[1].name = %q, want %q", tf.name, "terraform")
	}
	if tf.current != "1.9.8" {
		t.Errorf("items[1].current = %q, want %q", tf.current, "1.9.8")
	}

	// vault has a current version
	v := items[2].(appItem)
	if v.name != "vault" {
		t.Errorf("items[2].name = %q, want %q", v.name, "vault")
	}
	if v.current != "1.15.3" {
		t.Errorf("items[2].current = %q, want %q", v.current, "1.15.3")
	}
}

func TestBuildAppItemsFilterValue(t *testing.T) {
	apps := []string{"terraform"}
	items := buildAppItems(apps, nil)
	ai := items[0].(appItem)
	if ai.FilterValue() != "terraform" {
		t.Errorf("FilterValue() = %q, want %q", ai.FilterValue(), "terraform")
	}
}

// ---- buildVersionItems -----------------------------------------------------

func TestBuildVersionItemsEmpty(t *testing.T) {
	items := buildVersionItems(nil, nil, "")
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestBuildVersionItemsInstalledAndActive(t *testing.T) {
	versions := []string{"1.9.10", "1.9.9", "1.9.8"}
	installed := map[string]bool{
		"1.9.9": true,
		"1.9.8": true,
	}
	currentVer := "1.9.9"

	items := buildVersionItems(versions, installed, currentVer)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// 1.9.10: not installed, not active
	v0 := items[0].(versionItem)
	if v0.version != "1.9.10" {
		t.Errorf("items[0].version = %q, want %q", v0.version, "1.9.10")
	}
	if v0.installed {
		t.Error("items[0].installed should be false")
	}
	if v0.active {
		t.Error("items[0].active should be false")
	}

	// 1.9.9: installed and active
	v1 := items[1].(versionItem)
	if v1.version != "1.9.9" {
		t.Errorf("items[1].version = %q, want %q", v1.version, "1.9.9")
	}
	if !v1.installed {
		t.Error("items[1].installed should be true")
	}
	if !v1.active {
		t.Error("items[1].active should be true")
	}

	// 1.9.8: installed but not active
	v2 := items[2].(versionItem)
	if v2.version != "1.9.8" {
		t.Errorf("items[2].version = %q, want %q", v2.version, "1.9.8")
	}
	if !v2.installed {
		t.Error("items[2].installed should be true")
	}
	if v2.active {
		t.Error("items[2].active should be false")
	}
}

func TestBuildVersionItemsNilInstalledMap(t *testing.T) {
	versions := []string{"1.9.8"}
	items := buildVersionItems(versions, nil, "")
	vi := items[0].(versionItem)
	if vi.installed {
		t.Error("installed should be false when map is nil")
	}
	if vi.active {
		t.Error("active should be false when currentVer is empty")
	}
}

func TestBuildVersionItemsFilterValue(t *testing.T) {
	items := buildVersionItems([]string{"1.9.8"}, nil, "")
	vi := items[0].(versionItem)
	if vi.FilterValue() != "1.9.8" {
		t.Errorf("FilterValue() = %q, want %q", vi.FilterValue(), "1.9.8")
	}
}

func castVersionItems(t *testing.T, items []list.Item) []versionItem {
	t.Helper()
	out := make([]versionItem, len(items))
	for i, raw := range items {
		vi, ok := raw.(versionItem)
		if !ok {
			t.Fatalf("items[%d] is not a versionItem", i)
		}
		out[i] = vi
	}
	return out
}

func TestBuildVersionItemsNoActiveVersion(t *testing.T) {
	versions := []string{"2.0.0", "1.9.8"}
	installed := map[string]bool{"1.9.8": true}
	items := castVersionItems(t, buildVersionItems(versions, installed, ""))
	if items[0].active || items[1].active {
		t.Error("no item should be active when currentVer is empty")
	}
}

// ---- parseYAMLView ---------------------------------------------------------

func TestParseYAMLViewEmpty(t *testing.T) {
	yv := parseYAMLView("")
	if yv == nil {
		t.Fatal("parseYAMLView returned nil")
	}
	// An empty string produces one empty line after splitting on "\n".
	if len(yv.lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(yv.lines))
	}
	if len(yv.selIdxs) != 0 {
		t.Errorf("expected 0 selectable indices, got %d", len(yv.selIdxs))
	}
}

func TestParseYAMLViewKeyValues(t *testing.T) {
	yaml := "application: terraform\nversion: 1.9.8\n"
	yv := parseYAMLView(yaml)
	if yv == nil {
		t.Fatal("parseYAMLView returned nil")
	}
	// Two non-empty lines.
	if len(yv.lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %v", len(yv.lines), yv.lines)
	}
	// Both lines have values, so both should be selectable.
	if len(yv.selIdxs) != 2 {
		t.Errorf("expected 2 selectable indices, got %d", len(yv.selIdxs))
	}
	if len(yv.selValues) != 2 {
		t.Errorf("expected 2 selectable values, got %d", len(yv.selValues))
	}
	if yv.selValues[0] != "terraform" {
		t.Errorf("selValues[0] = %q, want %q", yv.selValues[0], "terraform")
	}
	if yv.selValues[1] != "1.9.8" {
		t.Errorf("selValues[1] = %q, want %q", yv.selValues[1], "1.9.8")
	}
}

func TestParseYAMLViewListItems(t *testing.T) {
	yaml := "builds:\n  - url: https://example.com/terraform_1.9.8_linux_amd64.zip\n"
	yv := parseYAMLView(yaml)
	if yv == nil {
		t.Fatal("parseYAMLView returned nil")
	}
	// Expect 2 lines: the "builds:" key line and the list item line.
	if len(yv.lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %v", len(yv.lines), yv.lines)
	}
	// "builds:" has no value; the list item has a key-value so it should be selectable.
	if len(yv.selIdxs) != 1 {
		t.Errorf("expected 1 selectable index, got %d", len(yv.selIdxs))
	}
	if yv.selValues[0] != "https://example.com/terraform_1.9.8_linux_amd64.zip" {
		t.Errorf("selValues[0] = %q", yv.selValues[0])
	}
}

func TestParseYAMLViewCursorNavigation(t *testing.T) {
	yaml := "application: terraform\nversion: 1.9.8\n"
	yv := parseYAMLView(yaml)
	// Initial cursor position is 0.
	if yv.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", yv.cursor)
	}
	// Move forward by 1.
	yv.move(1)
	if yv.cursor != 1 {
		t.Errorf("after move(1) cursor = %d, want 1", yv.cursor)
	}
	// Move forward wraps around.
	yv.move(1)
	if yv.cursor != 0 {
		t.Errorf("after second move(1) cursor = %d, want 0 (wrap)", yv.cursor)
	}
	// Move backward wraps around.
	yv.move(-1)
	if yv.cursor != 1 {
		t.Errorf("after move(-1) from 0, cursor = %d, want 1 (wrap)", yv.cursor)
	}
}

func TestParseYAMLViewSelectedValue(t *testing.T) {
	yaml := "application: terraform\nversion: 1.9.8\n"
	yv := parseYAMLView(yaml)
	if yv.selectedValue() != "terraform" {
		t.Errorf("selectedValue() = %q, want %q", yv.selectedValue(), "terraform")
	}
	yv.move(1)
	if yv.selectedValue() != "1.9.8" {
		t.Errorf("after move(1), selectedValue() = %q, want %q", yv.selectedValue(), "1.9.8")
	}
}

func TestParseYAMLViewCursorLineIdx(t *testing.T) {
	yaml := "application: terraform\nversion: 1.9.8\n"
	yv := parseYAMLView(yaml)
	// The first selectable line should be line index 0.
	if yv.cursorLineIdx() != 0 {
		t.Errorf("cursorLineIdx() = %d, want 0", yv.cursorLineIdx())
	}
}

func TestParseYAMLViewNoSelectables(t *testing.T) {
	// A YAML structure with only keys and no values (e.g. a map header).
	yaml := "builds:\n"
	yv := parseYAMLView(yaml)
	if len(yv.selIdxs) != 0 {
		t.Errorf("expected 0 selectable indices for key-only YAML, got %d", len(yv.selIdxs))
	}
	// With no selectables, cursorLineIdx returns -1.
	if yv.cursorLineIdx() != -1 {
		t.Errorf("cursorLineIdx() = %d, want -1", yv.cursorLineIdx())
	}
	// selectedValue returns "".
	if yv.selectedValue() != "" {
		t.Errorf("selectedValue() = %q, want empty", yv.selectedValue())
	}
	// move is a no-op and doesn't panic.
	yv.move(1)
	yv.move(-1)
}

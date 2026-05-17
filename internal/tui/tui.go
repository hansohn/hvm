// Package tui provides an interactive terminal UI for browsing and managing HashiCorp releases.
package tui

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hansohn/hvm/internal/install"
	"github.com/hansohn/hvm/internal/output"
	"github.com/hansohn/hvm/internal/releases"
	"gopkg.in/yaml.v3"
)

// stage represents the current navigation level in the TUI.
type stage string

const (
	stageApp      stage = "app"
	stageVersion  stage = "version"
	stageMetadata stage = "metadata"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	activeStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	installedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	dimStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	statusOKStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	statusErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

var (
	yamlKeyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	yamlValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	yamlDashStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	yamlColonStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	yamlCursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	yamlCopyMsgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
)

var (
	yamlListItemRe = regexp.MustCompile(`^(\s*)(-)(\s+)(.*)$`)
	yamlKeyValueRe = regexp.MustCompile(`^(\s*)([a-zA-Z][a-zA-Z0-9_-]*)(:\s*)(.*)$`)
)

// ---- item types ------------------------------------------------------------

type appItem struct {
	name    string
	current string // active version, "" if none
}

func (i appItem) FilterValue() string { return i.name }

type versionItem struct {
	version   string
	installed bool
	active    bool
}

func (i versionItem) FilterValue() string { return i.version }

// ---- delegates -------------------------------------------------------------

type appDelegate struct{}

func (d appDelegate) Height() int                             { return 1 }
func (d appDelegate) Spacing() int                            { return 0 }
func (d appDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d appDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	ai, ok := listItem.(appItem)
	if !ok {
		return
	}
	str := fmt.Sprintf("%d. %s", index+1, ai.name)
	if ai.current != "" {
		str += " " + dimStyle.Render("(→ "+ai.current+")")
	}
	if index == m.Index() {
		fmt.Fprint(w, selectedItemStyle.Render("❯ "+str))
	} else {
		fmt.Fprint(w, itemStyle.Render(str))
	}
}

type versionDelegate struct{}

func (d versionDelegate) Height() int                             { return 1 }
func (d versionDelegate) Spacing() int                            { return 0 }
func (d versionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d versionDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	vi, ok := listItem.(versionItem)
	if !ok {
		return
	}
	var marker string
	switch {
	case vi.active:
		marker = activeStyle.Render("→") + " "
	case vi.installed:
		marker = installedStyle.Render("✓") + " "
	default:
		marker = "  "
	}
	str := fmt.Sprintf("%d. %s%s", index+1, marker, vi.version)
	if index == m.Index() {
		fmt.Fprint(w, selectedItemStyle.Render("❯ "+str))
	} else {
		fmt.Fprint(w, itemStyle.Render(str))
	}
}

// ---- tea messages ----------------------------------------------------------

type opDoneMsg struct {
	version string
	err     error
}

// ---- yaml view -------------------------------------------------------------

type yamlView struct {
	lines     []string
	selIdxs   []int
	selValues []string
	cursor    int
}

func (v *yamlView) cursorLineIdx() int {
	if len(v.selIdxs) == 0 {
		return -1
	}
	return v.selIdxs[v.cursor]
}

func (v *yamlView) move(delta int) {
	if len(v.selIdxs) == 0 {
		return
	}
	v.cursor = (v.cursor + delta + len(v.selIdxs)) % len(v.selIdxs)
}

func (v *yamlView) selectedValue() string {
	if len(v.selValues) == 0 {
		return ""
	}
	return v.selValues[v.cursor]
}

// ---- model -----------------------------------------------------------------

type model struct {
	list     list.Model
	quitting bool
	stage    stage
	app      string
	version  string
	apps     []string
	versions []string
	metadata *releases.VersionMetadata
	err      error
	client   *releases.Client
	mgr      *install.Manager
	yv       *yamlView
	copyMsg  string

	// install state
	installed   map[string]bool
	currentVer  string
	appCurrents map[string]string

	// async install
	spin       spinner.Model
	loading    bool
	loadingMsg string

	// feedback
	statusMsg   string
	statusIsErr bool

	// remove confirmation
	confirmMode bool
	confirmVer  string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		if m.loading {
			return m, cmd
		}
		return m, nil

	case opDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
			m.statusIsErr = true
		} else {
			m.installed[msg.version] = true
			m.currentVer = msg.version
			m.appCurrents[m.app] = msg.version
			m.statusMsg = fmt.Sprintf("✓ Installed %s@%s", m.app, msg.version)
			m.statusIsErr = false
			m.list.SetItems(buildVersionItems(m.versions, m.installed, m.currentVer))
		}
		return m, nil

	case tea.KeyMsg:
		// Always allow quit.
		if msg.String() == "ctrl+c" || (msg.String() == "q" && !m.list.SettingFilter()) {
			m.quitting = true
			return m, tea.Quit
		}

		// Block all other keys during async download.
		if m.loading {
			return m, nil
		}

		switch msg.String() {
		case "esc", "n":
			if m.confirmMode {
				m.confirmMode = false
				m.confirmVer = ""
				return m, nil
			}

		case "b":
			if m.confirmMode {
				m.confirmMode = false
				m.confirmVer = ""
				return m, nil
			}
			return m.handleBack()

		case "a":
			if m.stage != stageApp && !m.confirmMode {
				return m.navigateToApps()
			}

		case "up", "k":
			if m.stage == stageMetadata && m.yv != nil {
				m.yv.move(-1)
				m.copyMsg = ""
				return m, nil
			}

		case "down", "j":
			if m.stage == stageMetadata && m.yv != nil {
				m.yv.move(1)
				m.copyMsg = ""
				return m, nil
			}

		case "i":
			if m.stage == stageVersion && !m.confirmMode {
				return m.doInstall()
			}

		case "u":
			if m.stage == stageVersion && !m.confirmMode {
				return m.doUse()
			}

		case "x":
			if m.stage == stageVersion && !m.confirmMode {
				return m.doConfirmRemove()
			}

		case "y":
			if m.confirmMode {
				return m.doRemove()
			}
			if m.stage == stageMetadata && m.yv != nil {
				if val := m.yv.selectedValue(); val != "" {
					m.copyMsg = writeClipboard(val)
				}
				return m, nil
			}

		case "enter":
			if m.confirmMode {
				return m.doRemove()
			}
			if m.stage == stageMetadata {
				if m.yv != nil {
					if val := m.yv.selectedValue(); val != "" {
						m.copyMsg = writeClipboard(val)
					}
				}
				return m, nil
			}
			return m.handleSelectCurrent()
		}
	}

	if m.stage != stageMetadata && !m.confirmMode {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

// ---- navigation ------------------------------------------------------------

func (m model) handleBack() (tea.Model, tea.Cmd) {
	switch m.stage {
	case stageMetadata:
		m.list = newVersionList(m.versions, m.installed, m.currentVer, m.app)
		m.stage = stageVersion
		m.metadata = nil
		m.yv = nil
		m.copyMsg = ""
		return m, nil
	case stageVersion:
		return m.navigateToApps()
	}
	return m, nil
}

func (m model) navigateToApps() (tea.Model, tea.Cmd) {
	m.list = newAppList(m.apps, m.appCurrents)
	m.stage = stageApp
	m.app = ""
	m.version = ""
	m.versions = nil
	m.metadata = nil
	m.yv = nil
	m.copyMsg = ""
	m.statusMsg = ""
	m.installed = nil
	m.currentVer = ""
	m.confirmMode = false
	m.confirmVer = ""
	return m, nil
}

func (m model) handleSelectCurrent() (tea.Model, tea.Cmd) {
	switch m.stage {
	case stageApp:
		if ai, ok := m.list.SelectedItem().(appItem); ok {
			return m.handleSelectApp(ai.name)
		}
	case stageVersion:
		if vi, ok := m.list.SelectedItem().(versionItem); ok {
			return m.handleSelectVersion(vi.version)
		}
	}
	return m, nil
}

func (m model) handleSelectApp(app string) (tea.Model, tea.Cmd) {
	versions, err := m.client.FetchVersions(app)
	if err != nil {
		m.err = err
		m.quitting = true
		return m, tea.Quit
	}
	m.app = app
	m.versions = versions
	m.stage = stageVersion
	m.statusMsg = ""
	m.confirmMode = false
	m.confirmVer = ""

	m.installed = map[string]bool{}
	if ivs, err := m.mgr.InstalledVersions(app); err == nil {
		for _, v := range ivs {
			m.installed[v] = true
		}
	}
	m.currentVer, _ = m.mgr.CurrentVersion(app, runtime.GOOS)
	m.list = newVersionList(versions, m.installed, m.currentVer, app)
	return m, nil
}

func (m model) handleSelectVersion(version string) (tea.Model, tea.Cmd) {
	metadata, err := m.client.FetchVersionMetadata(m.app, version)
	if err != nil {
		m.err = err
		m.quitting = true
		return m, tea.Quit
	}
	m.version = version
	m.metadata = metadata
	m.stage = stageMetadata
	m.yv = m.buildYAMLView()
	m.copyMsg = ""
	return m, nil
}

// ---- install / use / remove ------------------------------------------------

func (m model) doInstall() (tea.Model, tea.Cmd) {
	vi, ok := m.list.SelectedItem().(versionItem)
	if !ok {
		return m, nil
	}
	m.loading = true
	m.statusMsg = ""
	m.statusIsErr = false
	if vi.installed {
		m.loadingMsg = fmt.Sprintf("Activating %s@%s...", m.app, vi.version)
	} else {
		m.loadingMsg = fmt.Sprintf("Downloading %s@%s...", m.app, vi.version)
	}
	return m, tea.Batch(m.spin.Tick, m.installCmd(vi.version))
}

func (m model) doUse() (tea.Model, tea.Cmd) {
	vi, ok := m.list.SelectedItem().(versionItem)
	if !ok {
		return m, nil
	}
	if !vi.installed {
		m.statusMsg = vi.version + " is not installed — press i to install"
		m.statusIsErr = true
		return m, nil
	}
	if err := m.mgr.Use(m.app, vi.version, runtime.GOOS); err != nil {
		m.statusMsg = "Error: " + err.Error()
		m.statusIsErr = true
		return m, nil
	}
	m.currentVer = vi.version
	m.appCurrents[m.app] = vi.version
	m.statusMsg = fmt.Sprintf("Now using %s@%s", m.app, vi.version)
	m.statusIsErr = false
	m.list.SetItems(buildVersionItems(m.versions, m.installed, m.currentVer))
	return m, nil
}

func (m model) doConfirmRemove() (tea.Model, tea.Cmd) {
	vi, ok := m.list.SelectedItem().(versionItem)
	if !ok {
		return m, nil
	}
	if !vi.installed {
		m.statusMsg = vi.version + " is not installed"
		m.statusIsErr = true
		return m, nil
	}
	m.confirmMode = true
	m.confirmVer = vi.version
	m.statusMsg = ""
	return m, nil
}

func (m model) doRemove() (tea.Model, tea.Cmd) {
	if err := m.mgr.Remove(m.app, m.confirmVer, runtime.GOOS); err != nil {
		m.statusMsg = "Error: " + err.Error()
		m.statusIsErr = true
	} else {
		delete(m.installed, m.confirmVer)
		if m.currentVer == m.confirmVer {
			m.currentVer = ""
			m.appCurrents[m.app] = ""
		}
		m.statusMsg = fmt.Sprintf("Removed %s@%s", m.app, m.confirmVer)
		m.statusIsErr = false
		m.list.SetItems(buildVersionItems(m.versions, m.installed, m.currentVer))
	}
	m.confirmMode = false
	m.confirmVer = ""
	return m, nil
}

// installCmd runs the download + activate in a goroutine and returns an opDoneMsg.
func (m model) installCmd(version string) tea.Cmd {
	app := m.app
	mgr := m.mgr
	client := m.client
	targetOS := runtime.GOOS
	targetArch := runtime.GOARCH

	return func() tea.Msg {
		if !mgr.IsInstalled(app, version, targetOS) {
			metadata, err := client.FetchVersionMetadata(app, version)
			if err != nil {
				return opDoneMsg{version: version, err: err}
			}
			builds, _ := releases.FilterAndSortBuilds(metadata.Builds, targetOS, targetArch)
			if len(builds) == 0 {
				return opDoneMsg{version: version, err: fmt.Errorf("no build for %s/%s", targetOS, targetArch)}
			}
			if err := mgr.Download(app, version, targetOS, builds[0].URL); err != nil {
				return opDoneMsg{version: version, err: err}
			}
		}
		if err := mgr.Use(app, version, targetOS); err != nil {
			return opDoneMsg{version: version, err: err}
		}
		return opDoneMsg{version: version}
	}
}

// ---- views -----------------------------------------------------------------

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nError: %v\n", m.err)
	}
	if m.quitting {
		return "\n"
	}
	switch m.stage {
	case stageMetadata:
		return m.metadataView()
	case stageVersion:
		return m.versionView()
	default:
		return m.appView()
	}
}

func (m model) appView() string {
	return "\n" + m.list.View() +
		"\n  q: quit • ↑/↓: navigate • /: search • enter: select"
}

func (m model) versionView() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(m.list.View())
	sb.WriteString("\n")

	switch {
	case m.loading:
		sb.WriteString(fmt.Sprintf("\n  %s %s", m.spin.View(), m.loadingMsg))
	case m.confirmMode:
		sb.WriteString(fmt.Sprintf("\n  Remove %s@%s? (y/n)", m.app, m.confirmVer))
	case m.statusMsg != "":
		if m.statusIsErr {
			sb.WriteString("\n  " + statusErrStyle.Render(m.statusMsg))
		} else {
			sb.WriteString("\n  " + statusOKStyle.Render(m.statusMsg))
		}
	}

	sb.WriteString("\n  q: quit • b: back • ↑/↓: navigate • /: search • enter: view • i: install • u: use • x: remove\n")
	return sb.String()
}

func (m model) metadataView() string {
	cursorIdx := m.yv.cursorLineIdx()
	var sb strings.Builder
	sb.WriteString("\n")
	for i, line := range m.yv.lines {
		if i == cursorIdx {
			sb.WriteString(yamlCursorStyle.Render("❯") + " " + line + "\n")
		} else {
			sb.WriteString("  " + line + "\n")
		}
	}
	if m.copyMsg != "" {
		sb.WriteString("\n  " + yamlCopyMsgStyle.Render(m.copyMsg) + "\n")
	}
	sb.WriteString("\n  q: quit • b: back to versions • a: back to apps • ↑/↓: navigate • y/enter: copy\n")
	return sb.String()
}

// ---- Run -------------------------------------------------------------------

// Run starts the interactive TUI.
func Run(client *releases.Client) error {
	apps, err := client.FetchApplications()
	if err != nil {
		return fmt.Errorf("fetching applications: %w", err)
	}
	if len(apps) == 0 {
		return fmt.Errorf("no applications found")
	}

	mgr, err := install.New()
	if err != nil {
		return fmt.Errorf("initializing install manager: %w", err)
	}

	// Pre-load current versions for any locally installed apps.
	appCurrents := make(map[string]string)
	if installedApps, err := mgr.InstalledApps(); err == nil {
		for _, app := range installedApps {
			if ver, err := mgr.CurrentVersion(app, runtime.GOOS); err == nil && ver != "" {
				appCurrents[app] = ver
			}
		}
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := model{
		list:        newAppList(apps, appCurrents),
		stage:       stageApp,
		client:      client,
		mgr:         mgr,
		apps:        apps,
		appCurrents: appCurrents,
		spin:        sp,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	if fm, ok := final.(model); ok && fm.err != nil {
		return fm.err
	}
	return nil
}

// ---- list builders ---------------------------------------------------------

func buildAppItems(apps []string, appCurrents map[string]string) []list.Item {
	items := make([]list.Item, len(apps))
	for i, app := range apps {
		var current string
		if appCurrents != nil {
			current = appCurrents[app]
		}
		items[i] = appItem{name: app, current: current}
	}
	return items
}

func buildVersionItems(versions []string, installed map[string]bool, currentVer string) []list.Item {
	items := make([]list.Item, len(versions))
	for i, v := range versions {
		items[i] = versionItem{
			version:   v,
			installed: installed != nil && installed[v],
			active:    v == currentVer,
		}
	}
	return items
}

func newAppList(apps []string, appCurrents map[string]string) list.Model {
	return newBaseList(buildAppItems(apps, appCurrents), appDelegate{}, "Select a HashiCorp application")
}

func newVersionList(versions []string, installed map[string]bool, currentVer, app string) list.Model {
	return newBaseList(
		buildVersionItems(versions, installed, currentVer),
		versionDelegate{},
		fmt.Sprintf("Select a version for %s", app),
	)
}

func newBaseList(items []list.Item, delegate list.ItemDelegate, title string) list.Model {
	l := list.New(items, delegate, 20, 14)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	return l
}

// ---- helpers ---------------------------------------------------------------

func (m model) buildYAMLView() *yamlView {
	current, _ := releases.FilterAndSortBuilds(m.metadata.Builds, runtime.GOOS, runtime.GOARCH)
	if current == nil {
		current = []releases.Build{}
	}
	result := output.VersionOutput{
		Application: m.app,
		Version:     m.version,
		URL:         fmt.Sprintf("%s/%s/%s/", m.client.BaseURL, m.app, m.version),
		Platform:    &output.PlatformInfo{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Metadata: &releases.VersionMetadata{
			Builds: current,
			Files:  releases.FilterFilesByPlatform(m.metadata.Files, runtime.GOOS, runtime.GOARCH),
		},
	}
	b, err := yaml.Marshal(result)
	if err != nil {
		return &yamlView{lines: []string{"error: " + err.Error()}}
	}
	return parseYAMLView(string(b))
}

func parseYAMLView(yamlStr string) *yamlView {
	raw := strings.Split(strings.TrimRight(yamlStr, "\n"), "\n")
	v := &yamlView{lines: make([]string, len(raw))}
	for i, line := range raw {
		var coloredLine, value string
		if m := yamlListItemRe.FindStringSubmatch(line); m != nil {
			indent, dash, space, rest := m[1], m[2], m[3], m[4]
			coloredLine = indent + yamlDashStyle.Render(dash) + space + colorizeYAMLKeyValue(rest)
			if km := yamlKeyValueRe.FindStringSubmatch(rest); km != nil {
				value = km[4]
			} else {
				value = rest
			}
		} else {
			coloredLine = colorizeYAMLKeyValue(line)
			if km := yamlKeyValueRe.FindStringSubmatch(line); km != nil {
				value = km[4]
			}
		}
		v.lines[i] = coloredLine
		if value != "" {
			v.selIdxs = append(v.selIdxs, i)
			v.selValues = append(v.selValues, value)
		}
	}
	return v
}

func writeClipboard(val string) string {
	if err := clipboard.WriteAll(val); err == nil {
		return "✓ Copied: " + val
	}
	if _, err := osc52.New(val).WriteTo(os.Stderr); err == nil {
		return "✓ Copied: " + val
	}
	return "✗ Clipboard unavailable"
}

func colorizeYAMLKeyValue(s string) string {
	m := yamlKeyValueRe.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	indent, key, colon, value := m[1], m[2], m[3], m[4]
	colored := indent + yamlKeyStyle.Render(key) + yamlColonStyle.Render(colon)
	if value != "" {
		colored += yamlValueStyle.Render(value)
	}
	return colored
}

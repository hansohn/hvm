// Package main is the entry point for hvm, an interactive CLI for browsing
// and managing HashiCorp product releases.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/hansohn/hvm/internal/filter"
	"github.com/hansohn/hvm/internal/hvmrc"
	"github.com/hansohn/hvm/internal/install"
	"github.com/hansohn/hvm/internal/output"
	"github.com/hansohn/hvm/internal/releases"
	"github.com/hansohn/hvm/internal/tui"
	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags.
var version = "dev"

// Flag vars for list and get.
var (
	outputFmt   string
	releasesURL string
)

// Shared app query flag vars (list + get).
var (
	verFlag      string
	enterprise   bool
	hsm          bool
	preRelease   bool
	count        int
	osOverride   string
	archOverride string
	verbose      bool
	installed    bool
)

// get-only flag vars.
var noUse bool

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func warnIfNotInPath(binDir string) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == binDir {
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Warning: %s is not in your PATH\n", binDir)
	fmt.Fprintf(os.Stderr, "  Add the following to your shell profile:\n")
	fmt.Fprintf(os.Stderr, "  export PATH=\"%s:$PATH\"\n", binDir)
}

func newClient() *releases.Client {
	if releasesURL == "" {
		releasesURL = releases.DefaultBaseURL
	}
	return releases.New(releasesURL)
}

var rootCmd = &cobra.Command{
	Use:   "hvm",
	Short: "Interactive CLI for browsing and managing HashiCorp tool releases",
	Long: `hvm — HashiCorp Version Manager

Running hvm without arguments opens an interactive TUI for browsing and
installing HashiCorp tool releases. Use subcommands for non-interactive
and scripted workflows.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		return tui.Run(newClient())
	},
}

var listCmd = &cobra.Command{
	Use:           "list [app]",
	Short:         "List available applications or versions",
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runList,
}

var getCmd = &cobra.Command{
	Use:   "get <app>",
	Short: "Download and activate a HashiCorp tool",
	Long: `Download and activate a HashiCorp tool for the current platform.

If --version is omitted, hvm checks for an .hvmrc file starting in the current
directory and walking up to the filesystem root, then falls back to the latest
release. Pass --no-use to download without activating.

Example .hvmrc:
  terraform=1.9.8
  vault=1.15.3`,
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runGet,
}

var useCmd = &cobra.Command{
	Use:   "use [app] [version]",
	Short: "Activate an installed version",
	Long: `Activate an already-downloaded version by updating the symlink in ~/.hvm/bin.

If app is omitted, hvm reads the nearest .hvmrc and activates all listed apps.
If version is omitted, hvm checks for an .hvmrc file starting in the current
directory and walking up to the filesystem root.`,
	Args:          cobra.RangeArgs(0, 2),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runUse,
}

var currentCmd = &cobra.Command{
	Use:           "current <app>",
	Short:         "Show the currently active version",
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runCurrent,
}

var whichCmd = &cobra.Command{
	Use:           "which <app>",
	Short:         "Show the path to the active binary",
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runWhich,
}

var removeCmd = &cobra.Command{
	Use:           "remove <app> <version>",
	Short:         "Remove a downloaded version",
	Args:          cobra.ExactArgs(2),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runRemove,
}

var versionCmd = &cobra.Command{
	Use:           "version",
	Short:         "Print version information",
	SilenceUsage:  true,
	SilenceErrors: true,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("hvm version %s\n", version)
	},
}

func init() {
	listCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "Output format: json, yaml, or text")
	listCmd.Flags().StringVarP(&releasesURL, "mirror", "m", "", "Alternative releases URL for air-gapped environments")
	listCmd.Flags().StringVarP(&verFlag, "version", "v", "", "Version pattern (e.g., 1.9.8, 1.9, 1, or 'latest')")
	listCmd.Flags().BoolVar(&enterprise, "enterprise", false, "Show only enterprise versions (with +ent, excludes HSM)")
	listCmd.Flags().BoolVar(&hsm, "hsm", false, "Show only HSM versions (requires --enterprise)")
	listCmd.Flags().BoolVar(&preRelease, "pre-release", false, "Include pre-release versions (alpha, beta, rc)")
	listCmd.Flags().IntVarP(&count, "limit", "n", 10, "Number of versions to return (-1 for all)")
	listCmd.Flags().StringVar(&osOverride, "os", "", "Override OS for platform filtering (e.g., linux, darwin, windows)")
	listCmd.Flags().StringVar(&archOverride, "arch", "", "Override architecture for platform filtering (e.g., amd64, arm64, 386)")
	listCmd.Flags().BoolVar(&verbose, "verbose", false, "Show full metadata for each version")
	listCmd.Flags().BoolVar(&installed, "installed", false, "Show only locally installed versions")

	getCmd.Flags().StringVarP(&releasesURL, "mirror", "m", "", "Alternative releases URL for air-gapped environments")
	getCmd.Flags().StringVarP(&verFlag, "version", "v", "", "Version to download (default: .hvmrc or latest)")
	getCmd.Flags().BoolVar(&noUse, "no-use", false, "Download without activating")
	getCmd.Flags().BoolVar(&enterprise, "enterprise", false, "Resolve latest enterprise version")
	getCmd.Flags().BoolVar(&hsm, "hsm", false, "Resolve latest HSM version (requires --enterprise)")
	getCmd.Flags().BoolVar(&preRelease, "pre-release", false, "Include pre-release when resolving latest")
	getCmd.Flags().StringVar(&osOverride, "os", "", "Override OS (e.g., linux, darwin, windows)")
	getCmd.Flags().StringVar(&archOverride, "arch", "", "Override architecture (e.g., amd64, arm64)")

	rootCmd.AddCommand(listCmd, getCmd, useCmd, removeCmd, currentCmd, whichCmd, versionCmd)

	// Force cobra to register the completion command now so we can hide it from
	// help while keeping it usable via "hvm completion <shell>".
	rootCmd.InitDefaultCompletionCmd()
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "completion" {
			cmd.Hidden = true
		}
	}
}

func runList(_ *cobra.Command, args []string) error {
	if installed {
		mgr, err := install.New()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			installedApps, listErr := mgr.InstalledApps()
			if listErr != nil {
				return fmt.Errorf("listing installed apps: %w", listErr)
			}
			return output.Write(installedApps, outputFmt)
		}
		app := args[0]
		versions, versionsErr := mgr.InstalledVersions(app)
		if versionsErr != nil {
			return fmt.Errorf("listing installed versions: %w", versionsErr)
		}
		if outputFmt != "text" && outputFmt != "" {
			return output.Write(versions, outputFmt)
		}
		current, currentErr := mgr.CurrentVersion(app, runtime.GOOS)
		if currentErr != nil {
			current = ""
		}
		for _, v := range versions {
			if v == current {
				fmt.Printf("-> %s\n", v)
			} else {
				fmt.Printf("   %s\n", v)
			}
		}
		return nil
	}

	client := newClient()

	if len(args) == 0 {
		apps, err := client.FetchApplications()
		if err != nil {
			return fmt.Errorf("fetching applications: %w", err)
		}
		return output.Write(apps, outputFmt)
	}

	app := args[0]

	effectiveOS := runtime.GOOS
	if osOverride != "" {
		effectiveOS = osOverride
	}
	effectiveArch := runtime.GOARCH
	if archOverride != "" {
		effectiveArch = archOverride
	}

	versions, err := client.FetchVersions(app)
	if err != nil {
		return fmt.Errorf("fetching versions: %w", err)
	}
	versions = filter.PreReleaseVersions(versions, preRelease)
	versions = filter.EnterpriseVersions(versions, enterprise, hsm)

	if verFlag != "" {
		versions = filter.VersionsByPattern(verFlag, versions)
		if len(versions) == 0 {
			return fmt.Errorf("no version matching %q found for %s", verFlag, app)
		}
	}
	selected := filter.LimitVersionCount(versions, count)

	if !verbose {
		return output.Write(selected, outputFmt)
	}

	results := make([]output.VersionOutput, 0, len(selected))
	for _, ver := range selected {
		metadata, err := client.FetchVersionMetadata(app, ver)
		if err != nil {
			return fmt.Errorf("fetching metadata for %s: %w", ver, err)
		}
		currentBuilds, _ := releases.FilterAndSortBuilds(metadata.Builds, effectiveOS, effectiveArch)
		if currentBuilds == nil {
			currentBuilds = []releases.Build{}
		}
		results = append(results, output.VersionOutput{
			Application: app,
			Version:     ver,
			URL:         fmt.Sprintf("%s/%s/%s/", client.BaseURL, app, ver),
			Platform:    &output.PlatformInfo{OS: effectiveOS, Arch: effectiveArch},
			Metadata: &releases.VersionMetadata{
				Builds: currentBuilds,
				Files:  releases.FilterFilesByPlatform(metadata.Files, effectiveOS, effectiveArch),
			},
		})
	}
	return output.Write(results, outputFmt)
}

func runGet(_ *cobra.Command, args []string) error {
	app := args[0]
	client := newClient()

	versions, err := client.FetchVersions(app)
	if err != nil {
		return fmt.Errorf("fetching versions: %w", err)
	}
	versions = filter.PreReleaseVersions(versions, preRelease)
	versions = filter.EnterpriseVersions(versions, enterprise, hsm)

	pattern := verFlag
	if pattern == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr == nil {
			if v, file := hvmrc.Lookup(app, cwd); v != "" {
				fmt.Fprintf(os.Stderr, "Found %q in %s\n", app+"="+v, file)
				pattern = v
			}
		}
	}
	if pattern == "" {
		pattern = "latest"
	}
	matched := filter.VersionsByPattern(pattern, versions)
	if len(matched) == 0 {
		return fmt.Errorf("no version matching %q found for %s", pattern, app)
	}
	resolvedVersion := matched[0]

	effectiveOS := runtime.GOOS
	if osOverride != "" {
		effectiveOS = osOverride
	}
	effectiveArch := runtime.GOARCH
	if archOverride != "" {
		effectiveArch = archOverride
	}

	mgr, err := install.New()
	if err != nil {
		return err
	}

	if mgr.IsInstalled(app, resolvedVersion, effectiveOS) {
		fmt.Printf("%s@%s is already installed\n", app, resolvedVersion)
	} else {
		metadata, metaErr := client.FetchVersionMetadata(app, resolvedVersion)
		if metaErr != nil {
			return fmt.Errorf("fetching metadata: %w", metaErr)
		}
		builds, _ := releases.FilterAndSortBuilds(metadata.Builds, effectiveOS, effectiveArch)
		if len(builds) == 0 {
			return fmt.Errorf("no build found for %s/%s", effectiveOS, effectiveArch)
		}
		fmt.Printf("Downloading %s@%s (%s/%s)...\n", app, resolvedVersion, effectiveOS, effectiveArch)
		if dlErr := mgr.Download(app, resolvedVersion, effectiveOS, builds[0].URL); dlErr != nil {
			return fmt.Errorf("downloading: %w", dlErr)
		}
		fmt.Printf("Downloaded %s@%s\n", app, resolvedVersion)
	}

	if !noUse {
		if useErr := mgr.Use(app, resolvedVersion, effectiveOS); useErr != nil {
			return fmt.Errorf("activating: %w", useErr)
		}
		fmt.Printf("Now using %s@%s\n", app, resolvedVersion)
		warnIfNotInPath(mgr.BinDir())
	}

	return nil
}

func runUse(_ *cobra.Command, args []string) error {
	mgr, err := install.New()
	if err != nil {
		return err
	}

	// No app specified — activate everything in the nearest .hvmrc.
	if len(args) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		entries, file := hvmrc.LookupAll(cwd)
		if entries == nil {
			return fmt.Errorf("no .hvmrc file found (searched from %s)", cwd)
		}
		fmt.Fprintf(os.Stderr, "Using .hvmrc from %s\n", file)
		for app, ver := range entries {
			if err := mgr.Use(app, ver, runtime.GOOS); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s@%s: %v\n", app, ver, err)
				continue
			}
			fmt.Printf("Now using %s@%s\n", app, ver)
		}
		warnIfNotInPath(mgr.BinDir())
		return nil
	}

	app := args[0]
	ver := ""
	if len(args) == 2 {
		ver = args[1]
	} else {
		if cwd, err := os.Getwd(); err == nil {
			if v, file := hvmrc.Lookup(app, cwd); v != "" {
				fmt.Fprintf(os.Stderr, "Found %q in %s\n", app+"="+v, file)
				ver = v
			}
		}
		if ver == "" {
			return fmt.Errorf("no version specified and no .hvmrc entry found for %s", app)
		}
	}

	if err := mgr.Use(app, ver, runtime.GOOS); err != nil {
		return err
	}
	fmt.Printf("Now using %s@%s\n", app, ver)
	warnIfNotInPath(mgr.BinDir())
	return nil
}

func runCurrent(_ *cobra.Command, args []string) error {
	app := args[0]
	mgr, err := install.New()
	if err != nil {
		return err
	}
	ver, err := mgr.CurrentVersion(app, runtime.GOOS)
	if err != nil {
		return err
	}
	if ver == "" {
		fmt.Printf("No active version of %s\n", app)
		return nil
	}
	fmt.Println(ver)
	return nil
}

func runWhich(_ *cobra.Command, args []string) error {
	app := args[0]
	mgr, err := install.New()
	if err != nil {
		return err
	}
	ver, err := mgr.CurrentVersion(app, runtime.GOOS)
	if err != nil {
		return err
	}
	if ver == "" {
		return fmt.Errorf("no active version of %s", app)
	}
	fmt.Println(mgr.BinaryPath(app, ver, runtime.GOOS))
	return nil
}

func runRemove(_ *cobra.Command, args []string) error {
	app, ver := args[0], args[1]

	mgr, err := install.New()
	if err != nil {
		return err
	}
	if err := mgr.Remove(app, ver, runtime.GOOS); err != nil {
		return err
	}
	fmt.Printf("Removed %s@%s\n", app, ver)
	return nil
}

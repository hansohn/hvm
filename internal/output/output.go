// Package output formats and prints release data in json, yaml, or text.
package output

import (
	"encoding/json"
	"fmt"

	"github.com/hansohn/hvm/internal/releases"
	"gopkg.in/yaml.v3"
)

// VersionOutput is the structured result returned for a resolved version query.
type VersionOutput struct {
	Application string                    `json:"application" yaml:"application"`
	Version     string                    `json:"version"     yaml:"version"`
	URL         string                    `json:"url"         yaml:"url"`
	Platform    *PlatformInfo             `json:"platform,omitempty" yaml:"platform,omitempty"`
	Metadata    *releases.VersionMetadata `json:"metadata"    yaml:"metadata"`
}

// PlatformInfo holds the OS and architecture used for platform filtering.
type PlatformInfo struct {
	OS   string `json:"os"   yaml:"os"`
	Arch string `json:"arch" yaml:"arch"`
}

// Write formats data and prints it to stdout according to format.
// Supported formats: "json", "yaml", "text" (or "").
func Write(data any, format string) error {
	switch format {
	case "json":
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Println(string(b))

	case "yaml":
		b, err := yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("error marshaling YAML: %w", err)
		}
		fmt.Print(string(b))

	case "text", "":
		switch v := data.(type) {
		case []string:
			for _, item := range v {
				fmt.Println(item)
			}
		case VersionOutput:
			printVersionOutput(v)
		case []VersionOutput:
			for _, vo := range v {
				printVersionOutput(vo)
			}
		default:
			return fmt.Errorf("unsupported data type for text output")
		}

	default:
		return fmt.Errorf("unsupported output format: %s (valid options: json, yaml, text)", format)
	}
	return nil
}

func printVersionOutput(o VersionOutput) {
	fmt.Printf("\nApplication: %s\n", o.Application)
	fmt.Printf("Version: %s\n", o.Version)
	fmt.Printf("URL: %s\n", o.URL)

	if o.Platform != nil {
		fmt.Printf("\nPlatform:\n")
		fmt.Printf("  OS: %s\n", o.Platform.OS)
		fmt.Printf("  Architecture: %s\n", o.Platform.Arch)
	}

	if o.Metadata != nil {
		if len(o.Metadata.Builds) > 0 {
			fmt.Printf("\nBuilds:\n")
			for _, b := range o.Metadata.Builds {
				fmt.Printf("  - OS: %s, Arch: %s\n", b.OS, b.Arch)
				fmt.Printf("    URL: %s\n", b.URL)
			}
		}
		if len(o.Metadata.Files) > 0 {
			fmt.Printf("\nFiles:\n")
			for _, f := range o.Metadata.Files {
				fmt.Printf("  - %s\n", f)
			}
		}
	}
	fmt.Println()
}

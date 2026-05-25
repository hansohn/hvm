<div align="center">
  <h3>hvm</h3>
  <p>Version manager for HashiCorp tools</p>
  <p>
    <!-- Build Status -->
    <a href="https://github.com/hansohn/hvm/actions/workflows/ci.yml">
      <img src="https://img.shields.io/github/actions/workflow/status/hansohn/hvm/ci.yml?branch=main&style=for-the-badge">
    </a>
    <!-- Github Tag -->
    <a href="https://github.com/hansohn/hvm/tags/">
      <img src="https://img.shields.io/github/tag/hansohn/hvm.svg?style=for-the-badge">
    </a>
    <!-- License -->
    <a href="https://github.com/hansohn/hvm/blob/main/LICENSE">
      <img src="https://img.shields.io/github/license/hansohn/hvm.svg?style=for-the-badge">
    </a>
    <!-- LinkedIn -->
    <a href="https://linkedin.com/in/ryanhansohn">
      <img src="https://img.shields.io/badge/-LinkedIn-black.svg?style=for-the-badge&logo=linkedin&colorB=555">
    </a>
  </p>
</div>

## :sparkles: Features

- **Interactive TUI** — browse applications, versions, and metadata with keyboard-driven navigation
- **Version management** — install, activate, and remove HashiCorp tool versions
- **Smart platform detection** — automatically detects your OS and architecture
- **`.hvmrc` support** — pin tool versions per project, inspired by nvm
- **Flexible output** — text, JSON, and YAML output formats for scripting
- **Air-gapped support** — point to an internal mirror with `--mirror`

## :hammer_and_wrench: Installation

### Download a release (recommended)

Download the latest binary for your platform from the [releases page](https://github.com/hansohn/hvm/releases/latest), extract it, and move it to a directory on your `PATH`.

**macOS / Linux**

```bash
# macOS (Apple Silicon)
curl -L https://github.com/hansohn/hvm/releases/latest/download/hvm_$(curl -s https://api.github.com/repos/hansohn/hvm/releases/latest | grep tag_name | cut -d'"' -f4 | tr -d v)_darwin_arm64.tar.gz | tar -xz
# macOS (Intel)
curl -L https://github.com/hansohn/hvm/releases/latest/download/hvm_$(curl -s https://api.github.com/repos/hansohn/hvm/releases/latest | grep tag_name | cut -d'"' -f4 | tr -d v)_darwin_amd64.tar.gz | tar -xz
# Linux (amd64)
curl -L https://github.com/hansohn/hvm/releases/latest/download/hvm_$(curl -s https://api.github.com/repos/hansohn/hvm/releases/latest | grep tag_name | cut -d'"' -f4 | tr -d v)_linux_amd64.tar.gz | tar -xz

sudo mv hvm /usr/local/bin/
```

**Windows**

Download the `.zip` from the [releases page](https://github.com/hansohn/hvm/releases/latest), extract `hvm.exe`, and add it to a directory in your `%PATH%`.

### go install

If you have Go 1.21+ installed:

```bash
go install github.com/hansohn/hvm@latest
```

The binary is placed in `$GOPATH/bin` (typically `~/go/bin`). Ensure that directory is on your `PATH`. Note: the version reported by `hvm version` will show `dev` when installed this way — use a release download for a versioned binary.

### Build from source (native)

```bash
make go/build-local
# binary written to ./bin/hvm
```

### Build via Docker toolchain

```bash
make go/build
# cross-compiles for your local OS/arch inside Docker
# binary written to ./bin/hvm
```

### Add to PATH

```bash
# Add ~/.hvm/bin to your PATH so activated versions are available
export PATH="$HOME/.hvm/bin:$PATH"
```

Add that line to your shell profile (`.zshrc`, `.bashrc`, etc.) to make it permanent.

## :open_book: Usage

### Interactive TUI

Run without arguments to open the TUI:

```bash
hvm
```

**Keyboard controls:**

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate list |
| `Enter` | Select item |
| `/` | Filter/search |
| `Esc` | Clear filter / cancel |
| `i` | Install version |
| `u` | Use (activate) version |
| `x` | Remove version |
| `b` | Go back |
| `q` or `Ctrl+C` | Quit |

### Subcommands

#### `hvm list`

List available applications or versions for an app.

```bash
# List all available HashiCorp applications
hvm list

# List the 10 most recent Terraform versions (default limit)
hvm list terraform

# List all versions
hvm list terraform -n -1

# List a specific version (shows full metadata)
hvm list terraform -v 1.9.8

# List only installed versions
hvm list terraform --installed

# Enterprise and HSM versions
hvm list vault --enterprise
hvm list vault --enterprise --hsm

# Include pre-release versions
hvm list terraform --pre-release

# JSON or YAML output
hvm list terraform -o json
hvm list terraform -n 5 -o yaml
```

**Flags:**

| Flag | Description |
|------|-------------|
| `-v, --version` | Version pattern (e.g. `1.9.8`, `1.9`, `1`, `latest`) |
| `-n, --limit` | Number of versions to return, `-1` for all (default: `10`) |
| `--enterprise` | Show only enterprise versions (`+ent`) |
| `--hsm` | Show only HSM versions (requires `--enterprise`) |
| `--pre-release` | Include pre-release versions (alpha, beta, rc) |
| `--installed` | Show only locally installed versions |
| `--os` | Override OS (e.g. `linux`, `darwin`, `windows`) |
| `--arch` | Override architecture (e.g. `amd64`, `arm64`) |
| `--verbose` | Show full metadata for each version |
| `-o, --output` | Output format: `text`, `json`, or `yaml` (default: `text`) |
| `-m, --mirror` | Alternative releases URL for air-gapped environments |

#### `hvm get`

Download and activate a tool. Reads version from `.hvmrc` if `--version` is omitted, otherwise resolves `latest`.

```bash
# Download and activate the latest Terraform
hvm get terraform

# Download a specific version
hvm get terraform -v 1.9.8

# Download without activating
hvm get terraform -v 1.9.8 --no-use

# Download latest enterprise version
hvm get vault --enterprise

# Use an internal mirror
hvm get terraform -m http://internal-mirror.company.com/releases
```

#### `hvm use`

Activate an already-downloaded version by updating the symlink in `~/.hvm/bin`.

```bash
# Activate a specific version
hvm use terraform 1.9.8

# Activate from .hvmrc (reads nearest .hvmrc, activates all listed apps)
hvm use
```

#### `hvm current`

Show the currently active version for a tool.

```bash
hvm current terraform
# terraform 1.9.8
```

#### `hvm which`

Show the path to the active binary.

```bash
hvm which terraform
# /Users/you/.hvm/bin/terraform
```

#### `hvm remove`

Remove a downloaded version.

```bash
hvm remove terraform 1.9.8
```

### `.hvmrc`

Pin tool versions for a project by creating a `.hvmrc` file in the project directory:

```ini
# .hvmrc
terraform=1.9.8
vault=1.15.3
```

`hvm` walks up the directory tree from the current directory to find the nearest `.hvmrc`, the same way nvm works for Node.js.

```bash
# Download and activate all versions listed in .hvmrc
hvm get terraform   # picks up version from .hvmrc automatically
hvm get vault

# Activate all versions listed in .hvmrc at once
hvm use
```

### Version pattern matching

The `-v/--version` flag accepts flexible patterns:

| Pattern | Resolves to |
|---------|-------------|
| `latest` | Newest available version |
| `1.9.8` | Exact version |
| `1.9` | Latest `1.9.x` patch |
| `1` | Latest `1.x.x` minor |

### Air-gapped environments

Use `--mirror` to point to an internal mirror that mirrors the HashiCorp releases directory structure:

```bash
hvm list terraform -m http://internal-mirror.company.com/releases
hvm get terraform -v 1.9.8 -m http://internal-mirror.company.com/releases
```

The mirror must expose the same structure as `releases.hashicorp.com`:
- `{base}/` — application list
- `{base}/{app}/` — version list
- `{base}/{app}/{version}/` — build files

## :gear: How it works

`hvm` manages versions using symlinks:

```
~/.hvm/
  bin/
    terraform -> ../versions/terraform/1.9.8/terraform
    vault     -> ../versions/vault/1.15.3/vault
  versions/
    terraform/
      1.9.8/
        terraform
    vault/
      1.15.3/
        vault
```

Set `HVM_HOME` to override the default `~/.hvm` directory.

## :computer: Development

### Make targets

```bash
make go/build-local     # Build binary locally (native)
make go/build           # Build binary for local OS/arch (docker)
make go/test-local      # Run tests locally (native)
make go/test            # Run tests (docker)
make go/lint            # Lint Go code (docker)
make go/security        # Security analysis (docker)
make go/check           # Run all checks (lint, security, build, test)
make go/clean           # Clean Go build artifacts
```

### Running checks

```bash
# Run all CI checks locally via Docker
make go/check

# Quick local test run
make go/test-local
```

### Docker stages

The Dockerfile exposes individual stages you can target directly:

```bash
# Lint only
docker buildx build --file docker/Dockerfile --target lint .

# Tests with coverage
docker buildx build --file docker/Dockerfile --target test .

# Security analysis
docker buildx build --file docker/Dockerfile --target security .

# Build binary for local OS/arch
make go/build
```

Tool versions are pinned as build args and can be overridden:

```bash
docker buildx build \
  --build-arg GO_VERSION=1.24 \
  --build-arg GOLANGCI_VERSION=v1.64.8 \
  --build-arg GOSEC_VERSION=v2.22.4 \
  --build-arg GOTESTSUM_VERSION=v1.12.1 \
  --file docker/Dockerfile --target test .
```

## :package: Dependencies

- [Cobra](https://github.com/spf13/cobra) — CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — terminal styling
- [go-version](https://github.com/hashicorp/go-version) — semver sorting
- [golang.org/x/net/html](https://pkg.go.dev/golang.org/x/net/html) — HTML parsing

## :page_facing_up: License

Apache 2.0

## :warning: Disclaimer

This project is not affiliated with, endorsed by, or sponsored by HashiCorp, Inc. HashiCorp, Terraform, Vault, Consul, Nomad, and other HashiCorp product names are trademarks of HashiCorp, Inc. This tool simply automates downloading publicly available releases from HashiCorp's release server.

<!-- MARKDOWN LINKS & IMAGES -->
<!-- https://www.markdownguide.org/basic-syntax/#reference-style-links -->

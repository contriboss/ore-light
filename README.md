# ⚡️ Ore Light

> _The lean, adoption-friendly core of the Ore ecosystem._

Ore Light is the streamlined distribution of Ore – focused on fast gem installation, Bundler compatibility, and a welcoming feature set for new teams. It keeps Bundler in the loop while accelerating the painful parts with modern Go tooling.

## Why Ore Light?

- **Complete Bundler parity**: 21 commands covering all essential Bundler workflows
- **Multi-source support**: Install gems from rubygems.org, gem.coop, private servers, git repos, and local paths
- **Bundler-aware, not Bundler-bound**: Understands the Bundler ecosystem but performs downloads, caching, and installs without invoking `bundle install`
- **Fast by default**: Go's concurrency gives parallel downloads, connection pooling, and intelligent caching with zero Ruby requirement
- **Native extension support**: Automatically builds C/C++/Rust extensions supporting gems like nokogiri, pg, sqlite3
- **Security auditing**: Scan for vulnerabilities using bundler-audit's database (no Ruby required)
- **Dependency visualization**: Beautiful colored tree view of gem dependencies
- **Platform filtering**: Only downloads gems for your current platform (arm64-darwin, x86_64-linux, etc.)
- **Proper binstubs**: Generates Ruby wrapper scripts (not symlinks) that work without `bundle exec`
- **Group filtering**: Install production gems only with `--without development,test`
- **Modular foundation**: Built on extracted libraries (`gemfile-go`, `rubygems-client-go`) with PubGrub dependency resolution

## Quick Start

```bash
# Install Ore Light (no Ruby required for download)
# Installs to ~/.local/bin by default (no sudo needed)
curl -fsSL https://raw.githubusercontent.com/contriboss/ore-light/master/scripts/install.sh | bash

# For system-wide installation to /usr/local/bin
curl -fsSL https://raw.githubusercontent.com/contriboss/ore-light/master/scripts/install.sh | bash -s -- --system

# Install gems (automatically uses vendor/bundle/ruby/<version>)
ore install

# Run Ruby code with ore-managed environment
ore exec -- ruby -Iconfig -e "puts 'hello'"
```

### Typical Workflow

1. Use `ore fetch` or `ore install` to fetch gems in parallel.
2. Ore Light respects Bundler configuration:
   - If `.bundle/config` has a path configured (via `ore config --local path vendor/bundle`), gems install there
   - Otherwise, gems install to your system gem directory (same as regular `bundle install`)
3. Run `ore exec` (or use `bundle exec`) to execute commands with the correct gem paths.
4. For CI/CD isolation, configure a local vendor path: `ore config --local path vendor/bundle`

## Commands

Ore Light provides complete Bundler command parity with 21 commands:

**Project Setup:**
- `ore init` - Generate a new Gemfile

**Dependency Management:**
- `ore add` - Add gems to Gemfile (e.g., `ore add rails --version "~> 8.0"`)
- `ore remove` - Remove gems from Gemfile
- `ore update` - Update gems to their latest versions within constraints
- `ore lock` - Regenerate Gemfile.lock using the PubGrub resolver

**Information & Inspection:**
- `ore info` - Show detailed gem information (versions, dependencies)
- `ore list` - List all gems in the current bundle
- `ore outdated` - Show gems with newer versions available
- `ore show` - Show the source location of a gem
- `ore open` - Open a gem's source code in your editor
- `ore platform` - Display platform compatibility information
- `ore tree` - Display colorful dependency tree visualization

**Validation:**
- `ore check` - Verify all gems are installed
- `ore audit` - Scan for security vulnerabilities (bundler-audit compatible)
- `ore audit update` - Update vulnerability database
- `ore audit licenses` - Scan installed gems for license information

**Installation & Cleanup:**
- `ore fetch` - Prefetch gems (no Ruby required) and warm the cache
- `ore install` - Download and install gems with automatic native extension building
- `ore clean` - Remove unused gems from vendor directory
- `ore pristine` - Restore gems to pristine condition using `gem pristine` (requires Ruby)

**Execution:**
- `ore exec` - Run commands via `bundle exec` with ore-managed environment

**Configuration:**
- `ore config` - Get and set Bundler configuration options (works without Ruby/Bundler installed)

**Utilities:**
- `ore self-update` - Update ore to the latest version from GitHub releases
- `ore cache` - Inspect or prune the gem cache
- `ore stats` - Show Ruby environment statistics
- `ore why` - Show dependency chains for a gem
- `ore search` - Search for gems on RubyGems.org
- `ore gems` - List all installed gems in the system (with optional `--filter`)
- `ore browse` - Interactive TUI to browse, search, and manage installed gems
- `ore version` - Show version information

### Ruby Engine Compatibility

Ore Light automatically detects your Ruby engine and filters gems accordingly:

- **MRI/CRuby** - Full support for C extensions
- **JRuby** - Automatically skips C extension gems, allows JRuby-specific gems (jdbc-*, jar-dependencies)
- **TruffleRuby** - Full C extension support via LLVM

The engine is detected via `RUBY_ENGINE` environment variable or by running `ruby -e 'puts RUBY_ENGINE'`.

**Example:**
```bash
# JRuby automatically skips json, pg, nokogiri (C extensions)
# but allows jdbc-postgres, activerecord-jdbc-adapter
mise use jruby@10.0
ore install
```

### Native Extension Support

Ore Light automatically detects and builds native extensions when installing gems. It supports:

- **extconf.rb** - Traditional Ruby C extensions
- **CMakeLists.txt** - CMake-based extensions
- **Cargo.toml** - Rust-based extensions (via Magnus, Rutie, etc.)
- **configure** - Autotools-based extensions
- **Rakefile** - Rake-based compilation

**Requirements for building extensions:**
- Ruby installed and available in PATH
- C compiler (gcc, clang) for C/C++ extensions
- Rust toolchain for Rust extensions (if applicable)

**Skipping extensions:**
```bash
# Via flag
ore install --skip-extensions

# Via environment variable
export ORE_SKIP_EXTENSIONS=1
ore install
```

If Ruby is not available, Ore Light will automatically skip extension building with a warning.

### Dependency Visualization

View your gem dependencies as a colorful hierarchical tree:

```bash
ore tree
```

Features:
- Unicode box-drawing characters for clear hierarchy
- Color-coded gem names, versions, and platforms
- Shows groups (default, test, development)
- Platform indicators (e.g., `[arm64-darwin]`)
- Circular dependency detection
- Works with any TTY, falls back to plain text in pipes

### Self-Update

Keep ore up-to-date with built-in self-update functionality:

```bash
# Check for new versions
ore self-update --check

# Update to latest version (interactive)
ore self-update

# Update without confirmation
ore self-update --yes
```

Features:
- Automatic platform detection (macOS/Linux, amd64/arm64)
- SHA256 verification of downloads
- Atomic binary replacement with rollback support
- Interactive confirmation prompt
- Shows current → new version comparison
- Works without Ruby or any external tools

**Example:**
```
$ ore self-update
Checking target-arch... ore-v0.6.0-darwin-arm64.tar.gz
Checking current version... v0.5.1
Checking latest released version... v0.6.0
New release found! v0.5.1 --> v0.6.0

ore release status:
  * Current exe: "/usr/local/bin/ore"
  * New exe release: "ore-v0.6.0-darwin-arm64.tar.gz"
  * New exe download url: "https://github.com/contriboss/ore-light/releases/download/v0.6.0/ore_darwin_arm64.tar.gz"

The new release will be downloaded/extracted and the existing binary will be replaced.
Do you want to continue? [Y/n]
```

**Note:** Self-update requires ore to be installed from GitHub releases. Dev builds will show an error message.

### Security Auditing

Scan your gems for known security vulnerabilities using the same database as bundler-audit:

```bash
# Update the vulnerability database
ore audit update

# Scan Gemfile.lock for vulnerabilities
ore audit
```

Features:
- Compatible with bundler-audit's ruby-advisory-db
- No Ruby required for scanning
- Colorful output with severity levels (Critical/High/Medium/Low)
- Shows CVE numbers, affected versions, and solutions
- Database stored at `~/.local/share/ruby-advisory-db`

**Note:** This is a Go implementation extracted from ore_reference, providing the same workflow as bundler-audit without requiring Ruby.

### License Auditing

Scan your installed gems to see their license information, grouped by license type:

```bash
# Scan installed gems for licenses
ore audit licenses
```

Features:
- Reads license info from installed gemspecs using Ruby
- Groups gems by license type
- Color-coded output:
  - Green: Permissive licenses (MIT, Apache, BSD, ISC, Ruby, etc.)
  - Yellow with ⚠️: Copyleft licenses (GPL, AGPL, LGPL)
  - Red with ❌: Unknown or missing licenses
- Shows total gem count
- Helps identify licensing compliance issues

**Note:** Requires Ruby to read gemspec files. Run `ore install` first to ensure gems are installed.

### System Gem Browsing

Two commands for exploring all gems installed on your system (not just those in Gemfile.lock):

#### `ore gems` - List All Installed Gems

Simple command-line listing of all gems in your Ruby installation:

```bash
# List all installed gems
ore gems

# Filter by name
ore gems --filter rack
```

Features:
- Shows all system gems with versions
- Groups multiple versions of the same gem
- Color-coded output
- Total count summary

#### `ore browse` - Interactive TUI

Full-featured terminal UI for browsing and managing gems:

```bash
ore browse
```

Features:
- **Interactive list** with vim-style navigation (j/k to move)
- **Search/filter** - Press `/` to filter gems as you type
- **Quick actions:**
  - `o` - Open gem source in your editor
  - `i` - Show gem info (future: full gem details)
  - `w` - Show why gem is installed (future: dependency chain)
  - `q` or `Esc` - Quit
- **Real-time filtering** with instant results
- **Status bar** showing selected gem and keyboard shortcuts
- **Split view** ready for details panel (future enhancement)

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for a smooth TUI experience.

### Gem Source Fallback

Ore Light supports configuring multiple gem sources with automatic fallback when a primary source fails:

**Features:**
- Configure multiple gem sources with optional fallback URLs
- Automatic retry with fallback source on network errors, 5xx responses, or rate limiting (429)
- Support for authenticated sources (private gems, Sidekiq Pro, etc.)
- Pre-flight health checks to verify source availability before downloads
- Each source can have at most ONE fallback (no chaining)

**Configuration Example** (in `~/.config/ore/config.toml` or `.ore.toml`):
```toml
# Primary internal mirror, fallback to rubygems.org
[[gem_sources]]
url = "http://internal-mirror.company.com"
fallback = "https://rubygems.org"

# Private gems with authentication
[[gem_sources]]
url = "https://token:@gems.contribsys.com"  # Sidekiq Pro
fallback = "http://local-cache.dev"

# Additional source without fallback
[[gem_sources]]
url = "https://gem.coop"
```

**Authentication:**
- Token auth: `https://token:@gems.example.com`
- Basic auth: `https://username:password@gems.example.com`

When you run `ore install` or `ore fetch`, Ore Light will:
1. Perform health checks on all configured sources
2. Try downloading from the primary source
3. If a retryable error occurs and a fallback is configured, automatically switch to the fallback
4. Report which sources were used for successful downloads

### Configuration

#### Installation Path Priority

Ore Light determines where to install gems using this priority order:

1. **Environment variables**: `ORE_VENDOR_DIR` or `ORE_LIGHT_VENDOR_DIR`
2. **Ore config file**: `vendor_dir` in `.ore.toml` or `~/.config/ore/config.toml`
3. **Bundler config**: `BUNDLE_PATH` from `.bundle/config`
4. **System default**: Output of `gem environment gemdir`

**Configuration Examples:**

```bash
# Set install path without needing Ruby/Bundler installed
ore config --local path vendor/bundle
ore install

# Or use Bundler if you have it
bundle config set --local path vendor/bundle
ore install

# List current configuration
ore config --list

# Override with environment variable
ORE_VENDOR_DIR=/tmp/gems ore install
```

#### Configuration Files

Ore loads optional TOML configuration files:

- User config: `~/.config/ore/config.toml` (or `$XDG_CONFIG_HOME/ore/config.toml`)
- Project config: `./.ore.toml`

Supported keys:

```toml
vendor_dir = "/custom/path"
cache_dir = "/path/to/cache"
gemfile = "Gemfile.custom"

# Configure gem sources with optional fallbacks
[[gem_sources]]
url = "http://internal-mirror.company.com"
fallback = "https://rubygems.org"

[[gem_sources]]
url = "https://token:@gems.contribsys.com"  # Private gems (e.g., Sidekiq Pro)
fallback = "http://local-cache.dev"

[[gem_sources]]
url = "https://gem.coop"  # Standalone source without fallback
```

#### Environment Variables
- `ORE_SKIP_EXTENSIONS` / `ORE_LIGHT_SKIP_EXTENSIONS` - Set to `1`, `true`, or `yes` to skip native extension compilation
- `ORE_VENDOR_DIR` / `ORE_LIGHT_VENDOR_DIR` - Override default vendor directory
- `ORE_CACHE_DIR` / `ORE_LIGHT_CACHE_DIR` - Override default cache directory

## Relationship to `ore_reference`

The legacy repository now lives as `ore_reference`. It contains the full experimental feature surface, alternative providers, and advanced orchestration layers. Ore Light copies only the essentials needed for adoption, so the README, CLI surface, and docs will stay focused on the first run experience.

## Docker

Run ore-light in a container without installing Go or Rust:

```bash
# Basic usage (installs gems using Gemfile.lock)
docker run --rm -v $(pwd):/workspace ghcr.io/contriboss/ore-light:latest install

# With persistent cache
docker run --rm \
  -v $(pwd):/workspace \
  -v ore-cache:/cache \
  -e ORE_CACHE_DIR=/cache \
  ghcr.io/contriboss/ore-light:latest install

# Skip native extensions (no Ruby in image)
docker run --rm -v $(pwd):/workspace \
  ghcr.io/contriboss/ore-light:latest install --skip-extensions

# Check version
docker run --rm ghcr.io/contriboss/ore-light:latest version
```

**Local Development:**

```bash
# Build image locally
docker build -t ore-light:local .

# Test it
docker run --rm -v $(pwd):/workspace ore-light:local version

# Use docker-compose
docker-compose run --rm ore install
```

**Multi-architecture Build (for manual publishing):**

```bash
# Requires Docker Buildx
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ghcr.io/contriboss/ore-light:latest --push .
```

**Note:** The Docker image uses distroless base (~2MB) and doesn't include Ruby. For gems with native extensions, either use `--skip-extensions` flag or mount Ruby from the host system.

## GitHub Actions

Use ore in your CI/CD workflows with automatic caching for faster builds:

### Quick Start

```yaml
steps:
  # Step 1: Install ore (before Ruby setup)
  - uses: contriboss/ore-light/setup-ore@v1
    with:
      version: 'latest'  # or specific version like '0.1.0'

  # Step 2: Setup Ruby WITHOUT bundler caching
  - uses: ruby/setup-ruby@v1
    with:
      ruby-version: '3.4'
      bundler-cache: false  # Important: Let ore handle gems

  # Step 3: Install gems with ore (includes caching)
  - uses: contriboss/ore-light/ore-install@v1
```

### Actions Available

**`setup-ore`** - Installs and caches the ore binary
- Inputs: `version` (default: `latest`)
- Outputs: `version`, `cache-hit`
- Supports: Linux (amd64, arm64), macOS (amd64, arm64)

**`ore-install`** - Installs gems with intelligent caching
- Inputs: `working-directory`, `cache-key-prefix`, `skip-extensions`
- Outputs: `cache-hit`, `gems-installed`, `elapsed-time`
- Caches based on: `Gemfile.lock` hash + Ruby version + platform

### Full Example

```yaml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        ruby: ['3.2', '3.3', '3.4']

    steps:
      - uses: actions/checkout@v5

      - uses: contriboss/ore-light/setup-ore@v1

      - uses: ruby/setup-ruby@v1
        with:
          ruby-version: ${{ matrix.ruby }}
          bundler-cache: false  # Critical!

      - uses: contriboss/ore-light/ore-install@v1
        id: ore

      - name: Show stats
        run: |
          echo "Cache hit: ${{ steps.ore.outputs.cache-hit }}"
          echo "Gems installed: ${{ steps.ore.outputs.gems-installed }}"
          echo "Time: ${{ steps.ore.outputs.elapsed-time }}"

      - name: Run tests
        run: ore exec rake test
```

### Demo Workflow

See [.github/workflows/ore-demo.yml](.github/workflows/ore-demo.yml) for a complete working example you can trigger manually.

**Key Benefits:**
- ⚡ **Fast**: Parallel gem downloads + intelligent caching
- 🔄 **Compatible**: Works with existing Gemfile/Gemfile.lock
- 🚀 **Simple**: Drop-in replacement for `ruby/setup-ruby` bundler caching
- 🌍 **Multi-platform**: Linux and macOS support
- 📦 **No Ruby required**: ore binary is pure Go

## Development

```bash
mise install
mage build

# Install to ~/.local/bin (default if HOME is set)
mage install

# Or install to custom location
ORE_INSTALL_PREFIX=/usr/local/bin mage install

./bin/ore --help
```

**Installation behavior:**
- Defaults to `~/.local/bin` if `HOME` is set
- Falls back to `/usr/local/bin` if `HOME` is not set
- Override with `ORE_INSTALL_PREFIX` environment variable

## License

MIT

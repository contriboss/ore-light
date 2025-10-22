# ⚡️ Ore Light

> _The lean, adoption-friendly core of the Ore ecosystem._

Ore Light is the streamlined distribution of Ore – focused on fast gem installation, Bundler compatibility, and a welcoming feature set for new teams. It keeps Bundler in the loop while accelerating the painful parts with modern Go tooling.

## Why Ore Light?

- **Complete Bundler parity**: 17 commands covering all essential Bundler workflows
- **Multi-source support**: Install gems from rubygems.org, gem.coop, private servers, git repos, and local paths
- **Bundler-aware, not Bundler-bound**: Understands the Bundler ecosystem but performs downloads, caching, and installs without invoking `bundle install`
- **Fast by default**: Go's concurrency gives parallel downloads, connection pooling, and intelligent caching with zero Ruby requirement
- **Native extension support**: Automatically builds C/C++/Rust extensions supporting gems like nokogiri, pg, sqlite3
- **Proper binstubs**: Generates Ruby wrapper scripts (not symlinks) that work without `bundle exec`
- **Group filtering**: Install production gems only with `--without development,test`
- **Modular foundation**: Built on extracted libraries (`gemfile-go`, `rubygems-client-go`) with PubGrub dependency resolution

## Quick Start

```bash
# Install Ore Light (no Ruby required for download)
curl -Ls https://raw.githubusercontent.com/contriboss/ore-light/main/scripts/install.sh | bash

# Install gems (automatically uses vendor/bundle/ruby/<version>)
ore install

# Run Ruby code with ore-managed environment
ore exec -- ruby -Iconfig -e "puts 'hello'"
```

### Typical Workflow

1. Use `ore download` or `ore install` to fetch gems in parallel.
2. Ore Light respects Bundler configuration:
   - If `.bundle/config` has a path configured (via `ore config --local path vendor/bundle`), gems install there
   - Otherwise, gems install to your system gem directory (same as regular `bundle install`)
3. Run `ore exec` (or use `bundle exec`) to execute commands with the correct gem paths.
4. For CI/CD isolation, configure a local vendor path: `ore config --local path vendor/bundle`

## Commands

Ore Light provides complete Bundler command parity with 18 commands:

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
- `ore platform` - Display platform compatibility information

**Validation:**
- `ore check` - Verify all gems are installed

**Installation & Cleanup:**
- `ore download` - Prefetch gems (no Ruby required) and warm the cache
- `ore install` - Download and install gems with automatic native extension building
- `ore clean` - Remove unused gems from vendor directory

**Execution:**
- `ore exec` - Run commands via `bundle exec` with ore-managed environment

**Configuration:**
- `ore config` - Get and set Bundler configuration options (works without Ruby/Bundler installed)

**Utilities:**
- `ore cache` - Inspect or prune the gem cache
- `ore version` - Show version information

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
gem_mirror = "https://mirror.example.com"
gemfile = "Gemfile.custom"
```

#### Environment Variables
- `ORE_SKIP_EXTENSIONS` / `ORE_LIGHT_SKIP_EXTENSIONS` - Set to `1`, `true`, or `yes` to skip native extension compilation
- `ORE_VENDOR_DIR` / `ORE_LIGHT_VENDOR_DIR` - Override default vendor directory
- `ORE_CACHE_DIR` / `ORE_LIGHT_CACHE_DIR` - Override default cache directory
- `ORE_GEM_MIRROR` / `ORE_LIGHT_GEM_MIRROR` - Override gem download mirror

## Relationship to `ore_reference`

The legacy repository now lives as `ore_reference`. It contains the full experimental feature surface, alternative providers, and advanced orchestration layers. Ore Light copies only the essentials needed for adoption, so the README, CLI surface, and docs will stay focused on the first run experience.

## Development

```bash
mise install
mage build
# optionally install globally (requires write access to /usr/local/bin)
# mage install
./bin/ore --help
```

## License

MIT

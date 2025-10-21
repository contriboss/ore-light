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

# Warm the cache and unpack gems into vendor/ore
ore install --lockfile=Gemfile.lock --vendor=vendor/ore

# Run Ruby code with ore-managed environment
ore exec --vendor=vendor/ore -- ruby -Iconfig -e "puts 'hello'"
```

### Typical Workflow

1. Use `ore download` or `ore install` to fetch gems in parallel and warm the cache.
2. Ore Light unpacks gems into a vendor directory that can be mounted in CI/CD or Docker without requiring Ruby tooling.
3. Run `ore exec` (or your own Ruby entrypoint) with the vendor directory on the load path.
4. Rinse and repeat across CI/CD pipelines that previously required Ruby just to pull dependencies.

## Commands

Ore Light provides complete Bundler command parity with 17 commands:

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

Ore loads optional TOML configuration files with project settings overriding user settings:

- User config: `~/.config/ore/config.toml` (or `$XDG_CONFIG_HOME/ore/config.toml`)
- Project config: `./.ore.toml`

Command-line flags and environment variables still take precedence. Supported keys today:

```toml
vendor_dir = "vendor/ore"
cache_dir = "/path/to/cache"
gem_mirror = "https://mirror.example.com"
gemfile = "Gemfile.custom"
```

Environment variables:
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

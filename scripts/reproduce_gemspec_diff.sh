#!/bin/bash
set -e

GEM_NAME=${1:-"gitmoji-regex"}

# Determine script location and repo root
# This script is expected to be in <repo_root>/scripts/
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

# Temporary workspace
BASE_DIR="/tmp/ore_gemspec_repro_${GEM_NAME}_$(date +%s)"
ORE_BIN="$BASE_DIR/ore"

# Cleanup function
cleanup() {
  if [ -d "$BASE_DIR" ]; then
    echo "Cleaning up $BASE_DIR..."
    rm -rf "$BASE_DIR"
  fi
}
# Trap cleanup on exit (comment out if you want to inspect results after run)
# trap cleanup EXIT

echo "=== Setup ==="
echo "Target Gem: $GEM_NAME"
echo "Repo Root:  $REPO_ROOT"
echo "Workspace:  $BASE_DIR"

# Reset Bundler/Ruby environment variables to ensure clean slate
unset BUNDLE_GEMFILE
unset BUNDLE_PATH
unset GEM_PATH
unset GEM_HOME
unset RUBYLIB
export BUNDLE_IGNORE_CONFIG=1
export BUNDLE_USER_CACHE=0
export BUNDLE_USER_PLUGIN=0

mkdir -p "$BASE_DIR"

echo "=== Building Ore ==="
cd "$REPO_ROOT"
go build -o "$ORE_BIN" ./cmd/ore
ls -l "$ORE_BIN"
echo "Ore built at $ORE_BIN"

cd "$BASE_DIR"

echo "=== BUNDLER Install ==="
mkdir -p bundler_run/gems
cd bundler_run
echo "source 'https://rubygems.org'" > Gemfile
echo "gem '$GEM_NAME'" >> Gemfile

# Set local vendor path for Bundler
export GEM_HOME=$(pwd)/vendor
export GEM_PATH=$(pwd)/vendor
bundle config set --local path 'vendor'

# Run Bundle Install
bundle install

# Find the installed gemspec
# Note: Bundler installs specs to vendor/ruby/<ver>/specifications or vendor/specifications depending on config
BUNDLER_GEMSPEC=$(find vendor -name "${GEM_NAME}-*.gemspec" | head -n 1)
echo "Found Bundler gemspec at: $BUNDLER_GEMSPEC"

if [ -z "$BUNDLER_GEMSPEC" ]; then
  echo "Error: Could not find Bundler gemspec for $GEM_NAME"
  exit 1
fi

cp "$BUNDLER_GEMSPEC" ../${GEM_NAME}-bundler.gemspec
cd ..

echo "=== ORE Install ==="
mkdir -p ore_run/gems
cd ore_run
echo "source 'https://rubygems.org'" > Gemfile
echo "gem '$GEM_NAME'" >> Gemfile

# Ore uses GEM_HOME for installation target
export GEM_HOME=$(pwd)/vendor
"$ORE_BIN" install

# Find the valid installed gemspec (Ore might put it in vendor/specifications)
# Ensure we find the one Ore wrote, not a cached one if we were using cache (though workspace is clean)
ORE_GEMSPEC=$(find vendor -name "${GEM_NAME}-*.gemspec" | head -n 1)
echo "Found Ore gemspec at: $ORE_GEMSPEC"

if [ -z "$ORE_GEMSPEC" ]; then
  echo "Error: Could not find Ore gemspec for $GEM_NAME"
  exit 1
fi

cp "$ORE_GEMSPEC" ../${GEM_NAME}-ore.gemspec
cd ..

echo "=== COMPARISON ==="
echo "Diffing Bundler (left) vs Ore (right):"
diff -u ${GEM_NAME}-bundler.gemspec ${GEM_NAME}-ore.gemspec > diff.txt || true

if [ -s diff.txt ]; then
  cat diff.txt
  echo
  echo "FAILURE: Gemspecs differ."
  exit 1
else
  echo "SUCCESS: Gemspecs are identical!"
  exit 0
fi

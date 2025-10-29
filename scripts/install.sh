#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

# ore-light installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/contriboss/ore-light/main/scripts/install.sh | bash

REPO="contriboss/ore-light"
DEFAULT_SYSTEM_PREFIX="/usr/local/bin"

usage() {
  cat <<'EOF'
Usage: install.sh [options]

Options:
  --prefix <dir>    Install ore into the specified directory
  --system          Install into /usr/local/bin (prompts for sudo if needed)
  --version <tag>   Install a specific git tag (with or without leading v)
  --yes             Automatically answer yes to prompts (assumes sudo when required)
  --no-sudo         Never attempt to use sudo; fail if prefix is not writable
  -h, --help        Show this help text

Environment overrides:
  ORE_INSTALL_PREFIX       Install prefix override (same as --prefix)
  ORE_INSTALL_DIR          Legacy prefix override (kept for compatibility)
  ORE_INSTALL_VERSION      Version/tag to install (same as --version)
  ORE_INSTALL_ASSUME_SUDO  One of: always, never, prompt (default: prompt)

Without any options the script installs into ~/.local/bin, matching magefile defaults.
EOF
}

log() { printf '%s\n' "$*"; }
info() { printf '==> %s\n' "$*"; }
warn() { printf 'WARN: %s\n' "$*" >&2; }
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

expand_path() {
  local path="$1"
  case "$path" in
    "~")
      [ -n "${HOME:-}" ] || fail "HOME is not set; cannot expand ~"
      printf '%s\n' "$HOME"
      ;;
    "~/"*)
      [ -n "${HOME:-}" ] || fail "HOME is not set; cannot expand ~/"
      printf '%s/%s\n' "$HOME" "${path#~/}"
      ;;
    *)
      printf '%s\n' "$path"
      ;;
  esac
}

detect_http_client() {
  if command -v curl >/dev/null 2>&1; then
    HTTP_CLIENT="curl"
  elif command -v wget >/dev/null 2>&1; then
    HTTP_CLIENT="wget"
  else
    fail "curl or wget is required to download releases"
  fi
}

http_fetch() {
  local url="$1"
  if [ "$HTTP_CLIENT" = "curl" ]; then
    curl -fsSL --proto '=https' --tlsv1.2 "$url"
  else
    wget -qO- "$url"
  fi
}

http_download() {
  local url="$1"
  local dest="$2"
  if [ "$HTTP_CLIENT" = "curl" ]; then
    curl -fsSL --proto '=https' --tlsv1.2 -o "$dest" "$url"
  else
    wget -q -O "$dest" "$url"
  fi
}

parse_args() {
  REQUESTED_VERSION="${ORE_INSTALL_VERSION:-}"
  PREFIX_OVERRIDE="${ORE_INSTALL_PREFIX:-${ORE_INSTALL_DIR:-}}"
  SYSTEM_INSTALL=0
  ASSUME_YES=0
  CLI_SUDO_MODE=""

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --prefix)
        [ "$#" -ge 2 ] || fail "--prefix requires a directory argument"
        PREFIX_OVERRIDE="$2"
        shift
        ;;
      --version)
        [ "$#" -ge 2 ] || fail "--version requires a tag argument"
        REQUESTED_VERSION="$2"
        shift
        ;;
      --system)
        SYSTEM_INSTALL=1
        ;;
      --yes|--assume-yes)
        ASSUME_YES=1
        ;;
      --no-sudo)
        CLI_SUDO_MODE="never"
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "Unknown option: $1 (use --help for usage)"
        ;;
    esac
    shift
  done

  INSTALL_PREFIX="${PREFIX_OVERRIDE}"
  if [ -z "$INSTALL_PREFIX" ]; then
    if [ "$SYSTEM_INSTALL" -eq 1 ]; then
      INSTALL_PREFIX="$DEFAULT_SYSTEM_PREFIX"
    elif [ -n "${HOME:-}" ]; then
      INSTALL_PREFIX="$HOME/.local/bin"
    else
      INSTALL_PREFIX="$DEFAULT_SYSTEM_PREFIX"
    fi
  fi

  INSTALL_PREFIX=$(expand_path "$INSTALL_PREFIX")

  SUDO_MODE="prompt"
  case "${ORE_INSTALL_ASSUME_SUDO:-}" in
    always|never|prompt)
      SUDO_MODE="${ORE_INSTALL_ASSUME_SUDO}"
      ;;
    "")
      ;;
    *)
      warn "Ignoring invalid ORE_INSTALL_ASSUME_SUDO value: ${ORE_INSTALL_ASSUME_SUDO}"
      ;;
  esac

  if [ -n "$CLI_SUDO_MODE" ]; then
    SUDO_MODE="$CLI_SUDO_MODE"
  elif [ "$ASSUME_YES" -eq 1 ] && [ "$SUDO_MODE" = "prompt" ]; then
    SUDO_MODE="always"
  fi
}

detect_platform() {
  local os uname_s arch uname_m
  uname_s=$(uname -s 2>/dev/null || true)
  uname_m=$(uname -m 2>/dev/null || true)

  case "$(echo "$uname_s" | tr '[:upper:]' '[:lower:]')" in
    linux)
      os="linux"
      ;;
    darwin)
      os="darwin"
      ;;
    msys*|mingw*|cygwin*)
      fail "Windows is not supported by this installer; use the zip packages instead"
      ;;
    *)
      fail "Unsupported operating system: ${uname_s}"
      ;;
  esac

  case "$uname_m" in
    x86_64|amd64)
      arch="amd64"
      ;;
    aarch64|arm64)
      arch="arm64"
      ;;
    *)
      fail "Unsupported architecture: ${uname_m}"
      ;;
  esac

  OS="$os"
  ARCH="$arch"
}

resolve_tag() {
  if [ -n "$REQUESTED_VERSION" ]; then
    TAG="v${REQUESTED_VERSION#v}"
    return
  fi

  if ! release_json=$(http_fetch "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null); then
    warn "GitHub API unavailable; falling back to release redirect"
  else
    TAG=$(printf '%s\n' "$release_json" | awk -F'"' '/"tag_name":/ {print $4; exit}')
  fi

  if [ -z "${TAG:-}" ]; then
    if [ "$HTTP_CLIENT" != "curl" ]; then
      fail "Unable to determine latest release (curl required for fallback). Please install curl or specify --version."
    fi
    TAG=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)
    TAG="${TAG##*/}"
  fi

  if [ -z "${TAG:-}" ]; then
    fail "Could not determine latest release tag"
  fi
}

download_binary() {
  local tmp_dir tmp_bin download_url
  tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t ore-light-install)
  TMP_DIR="$tmp_dir"
  trap 'rm -rf "$TMP_DIR"' EXIT

  tmp_bin="${TMP_DIR}/ore"
  BINARY_NAME="ore-${OS}-${ARCH}"
  download_url="https://github.com/${REPO}/releases/download/${TAG}/${BINARY_NAME}"

  info "Downloading ore ${TAG} for ${OS}/${ARCH}"
  info "From: ${download_url}"

  if ! http_download "$download_url" "$tmp_bin"; then
    fail "Failed to download ${download_url}. Check the tag/architecture and try again."
  fi

  if [ ! -s "$tmp_bin" ]; then
    fail "Downloaded file is empty. The requested release may not provide ${BINARY_NAME}."
  fi

  chmod +x "$tmp_bin"
  DOWNLOADED_BIN="$tmp_bin"
}

ensure_install_prefix() {
  if [ "$(id -u)" -eq 0 ]; then
    USE_SUDO=0
    return
  fi

  if [ ! -d "$INSTALL_PREFIX" ]; then
    if mkdir -p "$INSTALL_PREFIX" 2>/dev/null; then
      USE_SUDO=0
      return
    fi
  elif [ -w "$INSTALL_PREFIX" ]; then
    USE_SUDO=0
    return
  else
    if touch "${INSTALL_PREFIX}/.ore-write-test" 2>/dev/null; then
      rm -f "${INSTALL_PREFIX}/.ore-write-test"
      USE_SUDO=0
      return
    fi
  fi

  USE_SUDO=1
}

prompt_for_sudo() {
  if [ "$USE_SUDO" -ne 1 ]; then
    return
  fi

  case "$SUDO_MODE" in
    never)
      fail "Install prefix ${INSTALL_PREFIX} is not writable and sudo use was disabled."
      ;;
    always)
      ;;
    prompt)
      if [ -t 0 ] && [ -t 1 ]; then
        printf 'Install prefix %s requires elevated privileges. Use sudo to continue? [y/N] ' "$INSTALL_PREFIX" >&2
        read -r reply
        case "$reply" in
          y|Y|yes|YES)
            ;;
          *)
            fail "Installation aborted at user request."
            ;;
        esac
      else
        fail "Cannot prompt for sudo in non-interactive mode. Re-run with --yes or choose a user-writable prefix."
      fi
      ;;
    *)
      fail "Unexpected sudo mode: ${SUDO_MODE}"
      ;;
  esac

  if ! command -v sudo >/dev/null 2>&1; then
    fail "sudo is required to install into ${INSTALL_PREFIX}. Install sudo or adjust the prefix."
  fi
}

install_binary() {
  local dest="${INSTALL_PREFIX}/ore"
  if [ "$USE_SUDO" -eq 1 ]; then
    sudo mkdir -p "$INSTALL_PREFIX"
    sudo install -m 0755 "$DOWNLOADED_BIN" "$dest"
  else
    mkdir -p "$INSTALL_PREFIX"
    install -m 0755 "$DOWNLOADED_BIN" "$dest"
  fi
  INSTALLED_PATH="$dest"
}

path_contains_dir() {
  local dir="$1"
  local old_ifs=$IFS entry
  IFS=":"
  for entry in ${PATH:-}; do
    if [ "$entry" = "$dir" ]; then
      IFS=$old_ifs
      return 0
    fi
  done
  IFS=$old_ifs
  return 1
}

print_completion() {
  info "Installed ore to ${INSTALLED_PATH}"
  if [ -x "$INSTALLED_PATH" ]; then
    local version_output
    if version_output="$("$INSTALLED_PATH" version 2>/dev/null)"; then
      log "$version_output"
    else
      warn "Installed binary could not report its version. Try running '${INSTALLED_PATH} version'."
    fi
  fi

  if path_contains_dir "$INSTALL_PREFIX"; then
    log "ore is ready to use. Try 'ore --help'."
  else
    warn "The directory ${INSTALL_PREFIX} is not on your PATH."
    if [ -n "${HOME:-}" ] && [[ "$INSTALL_PREFIX" == "$HOME/"* ]]; then
      local relative="${INSTALL_PREFIX#$HOME/}"
      cat <<EOF
Add the following line to your shell profile (e.g. ~/.bashrc or ~/.zshrc):
  export PATH="\$HOME/${relative}:\$PATH"

Then reload your shell or run: source ~/.bashrc
EOF
    else
      cat <<EOF
Add the following line to your shell profile (e.g. ~/.bashrc or ~/.zshrc):
  export PATH="${INSTALL_PREFIX}:\$PATH"

Then reload your shell or run: source ~/.bashrc
EOF
    fi
  fi
}

main() {
  command -v install >/dev/null 2>&1 || fail "'install' command not found. Please install coreutils/bsdinstall."
  detect_http_client
  parse_args "$@"
  detect_platform
  resolve_tag
  download_binary
  ensure_install_prefix
  prompt_for_sudo
  install_binary
  print_completion
}

main "$@"

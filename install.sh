#!/usr/bin/env bash
# Install / update Salad Terminal onto PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
#
# Downloads a prebuilt binary from GitHub Releases (no Go required).
# Contributors can force a source build with: SALAD_FORCE_SOURCE=1 ./install.sh
set -euo pipefail

REPO="${SALAD_TERMINAL_REPO:-adebayox/salad-terminal}"
RELEASE_TAG="${SALAD_TERMINAL_RELEASE:-latest}"
BASE_URL="${SALAD_TERMINAL_BASE_URL:-https://github.com/${REPO}/releases/download/${RELEASE_TAG}}"

need_cmd() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    return 0
  fi
  echo "error: need '$cmd' on PATH" >&2
  exit 1
}

resolve_bin_dir() {
  if [[ -n "${SALAD_BIN_DIR:-}" ]]; then
    mkdir -p "$SALAD_BIN_DIR"
    echo "$SALAD_BIN_DIR"
    return
  fi
  if [[ -w /usr/local/bin ]]; then
    echo /usr/local/bin
    return
  fi
  mkdir -p "${HOME}/.local/bin"
  echo "${HOME}/.local/bin"
}

install_binary() {
  local src="$1"
  local bin_dir
  bin_dir="$(resolve_bin_dir)"
  install -m 755 "$src" "${bin_dir}/salad"
  echo "Installed: ${bin_dir}/salad"

  case ":$PATH:" in
    *":${bin_dir}:"*) ;;
    *)
      echo
      echo "Add to your shell profile, then open a new terminal:"
      echo "  export PATH=\"${bin_dir}:\$PATH\""
      ;;
  esac
}

detect_target() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "$os" in
    darwin|linux) ;;
    *)
      echo "error: unsupported OS '$os' (need macOS or Linux)" >&2
      exit 1
      ;;
  esac
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *)
      echo "error: unsupported architecture '$arch'" >&2
      exit 1
      ;;
  esac
  echo "${os}-${arch}"
}

download_release_binary() {
  need_cmd curl
  local target asset url tmp ver
  target="$(detect_target)"
  asset="salad-${target}"
  url="${BASE_URL}/${asset}"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/salad-terminal.XXXXXX")"
  cleanup() { rm -rf "$tmp"; }
  trap cleanup EXIT

  echo "Downloading Salad Terminal (${RELEASE_TAG} / ${target})…"
  if ! curl -fsSL --connect-timeout 15 --max-time 120 -o "${tmp}/salad" "$url"; then
    echo "error: could not download ${url}" >&2
    echo >&2
    echo "Release binaries may still be publishing. Retry in a minute, or build from source:" >&2
    echo "  brew install go git" >&2
    echo "  SALAD_FORCE_SOURCE=1 curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | bash" >&2
    exit 1
  fi
  chmod +x "${tmp}/salad"

  ver="$(curl -fsSL --connect-timeout 10 --max-time 30 "${BASE_URL}/VERSION" 2>/dev/null || true)"
  if [[ -z "$ver" ]]; then
    ver="$RELEASE_TAG"
  fi

  install_binary "${tmp}/salad"
  echo "Version: ${ver}"
}

build_from_dir() {
  local root="$1"
  need_cmd go
  local ver
  ver="$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo dev)"
  echo "Building Salad Terminal from source (${ver})…"
  (cd "$root" && go build -ldflags "-s -w -X main.Version=${ver}" -o salad ./cmd/salad)
  install_binary "${root}/salad"
  echo "Version: ${ver}"
}

fetch_and_build_source() {
  need_cmd git
  need_cmd go
  local tmp
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/salad-terminal.XXXXXX")"
  cleanup() { rm -rf "$tmp"; }
  trap cleanup EXIT
  echo "Fetching Salad Terminal source…"
  git clone --depth 1 --branch "${SALAD_TERMINAL_REF:-main}" "https://github.com/${REPO}.git" "$tmp/src"
  build_from_dir "$tmp/src"
}

# curl|bash has no real script path — BASH_SOURCE is unbound/empty under `set -u`.
SCRIPT_PATH="${BASH_SOURCE[0]:-}"
SCRIPT_DIR=""
if [[ -n "$SCRIPT_PATH" && "$SCRIPT_PATH" != "bash" && "$SCRIPT_PATH" != "-" && -f "$SCRIPT_PATH" ]]; then
  SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
fi

if [[ "${SALAD_FORCE_SOURCE:-}" == "1" ]]; then
  if [[ -n "$SCRIPT_DIR" && -f "${SCRIPT_DIR}/go.mod" && -d "${SCRIPT_DIR}/cmd/salad" ]]; then
    build_from_dir "$SCRIPT_DIR"
  else
    fetch_and_build_source
  fi
elif [[ -n "$SCRIPT_DIR" && -f "${SCRIPT_DIR}/go.mod" && -d "${SCRIPT_DIR}/cmd/salad" && "${SALAD_FORCE_REMOTE:-}" != "1" ]]; then
  # Local checkout: prefer source build for contributors iterating.
  build_from_dir "$SCRIPT_DIR"
else
  download_release_binary
fi

echo
echo "Done. Next:"
echo "  salad login   # once"
echo "  salad"
echo "  salad update  # later"

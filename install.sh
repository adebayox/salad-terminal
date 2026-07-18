#!/usr/bin/env bash
# Install / update Salad Terminal onto PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
#   salad update
#
# Or from a local checkout: ./install.sh
set -euo pipefail

REPO_URL="${SALAD_TERMINAL_REPO:-https://github.com/adebayox/salad-terminal.git}"
REPO_REF="${SALAD_TERMINAL_REF:-main}"

need_cmd() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    return 0
  fi
  echo "error: need '$cmd' on PATH" >&2
  if [[ "$cmd" == "go" ]]; then
    echo >&2
    echo "Salad Terminal builds from source. Install Go, then re-run this installer:" >&2
    echo "  brew install go" >&2
    echo "  # or https://go.dev/dl/" >&2
    echo >&2
    echo "Then:" >&2
    echo "  curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash" >&2
  elif [[ "$cmd" == "git" ]]; then
    echo >&2
    echo "Install git, then re-run this installer:" >&2
    echo "  xcode-select --install" >&2
    echo "  # or: brew install git" >&2
  fi
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

build_from_dir() {
  local root="$1"
  need_cmd go
  local ver
  ver="$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo dev)"
  echo "Building Salad Terminal (${ver})…"
  (cd "$root" && go build -ldflags "-X main.Version=${ver}" -o salad ./cmd/salad)
  install_binary "${root}/salad"
  echo "Version: ${ver}"
}

fetch_and_build() {
  need_cmd git
  need_cmd go
  TMP="$(mktemp -d "${TMPDIR:-/tmp}/salad-terminal.XXXXXX")"
  cleanup() { rm -rf "$TMP"; }
  trap cleanup EXIT
  echo "Fetching Salad Terminal (${REPO_REF})…"
  git clone --depth 1 --branch "$REPO_REF" "$REPO_URL" "$TMP/src"
  build_from_dir "$TMP/src"
}

# curl|bash has no real script path — BASH_SOURCE is unbound/empty under `set -u`.
# Only build a local checkout when this file is executed from disk.
SCRIPT_PATH="${BASH_SOURCE[0]:-}"
SCRIPT_DIR=""
if [[ -n "$SCRIPT_PATH" && "$SCRIPT_PATH" != "bash" && "$SCRIPT_PATH" != "-" && -f "$SCRIPT_PATH" ]]; then
  SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
fi

if [[ "${SALAD_FORCE_REMOTE:-}" == "1" ]]; then
  fetch_and_build
elif [[ -n "$SCRIPT_DIR" && -f "${SCRIPT_DIR}/go.mod" && -d "${SCRIPT_DIR}/cmd/salad" ]]; then
  build_from_dir "$SCRIPT_DIR"
else
  fetch_and_build
fi

echo
echo "Done. Next:"
echo "  salad login   # once"
echo "  salad"
echo "  salad update  # later, when Terminal ships changes"

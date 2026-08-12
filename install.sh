#!/usr/bin/env bash
# Install / update Salad Terminal onto PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/adebayox/salad-terminal/main/install.sh | bash
#
# Downloads a compressed prebuilt binary from GitHub Releases (no Go required).
# Windows users should use install.ps1; this script deliberately fails closed.
# Contributors: SALAD_FORCE_SOURCE=1 ./install.sh
set -euo pipefail

REPO="${SALAD_TERMINAL_REPO:-adebayox/salad-terminal}"
RELEASE_TAG="${SALAD_TERMINAL_RELEASE:-latest}"

resolve_release_tag() {
  if [[ "$RELEASE_TAG" != "latest" ]]; then
    echo "$RELEASE_TAG"
    return
  fi
  need_cmd curl
  local payload tag
  payload="$(curl -fsSL --connect-timeout 10 --retry 3 --retry-all-errors "https://api.github.com/repos/${REPO}/releases/latest")"
  tag="$(printf '%s' "$payload" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  if [[ -z "$tag" ]]; then
    echo "error: could not resolve the latest immutable release for ${REPO}" >&2
    exit 1
  fi
  echo "$tag"
}

need_cmd() {
  local cmd="$1"
  if command -v "$cmd" >/dev/null 2>&1; then
    return 0
  fi
  echo "error: need '$cmd' on PATH" >&2
  exit 1
}

cleanup_dir() {
  local dir="$1"
  [[ -n "$dir" ]] && rm -rf -- "$dir"
}

SALAD_INSTALL_TMP_DIR=""

cleanup_install_tmp_dir() {
  if [[ -n "$SALAD_INSTALL_TMP_DIR" ]]; then
    cleanup_dir "$SALAD_INSTALL_TMP_DIR"
    SALAD_INSTALL_TMP_DIR=""
  fi
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
    mingw*|msys*|cygwin*)
      echo "error: use install.ps1 on Windows: https://raw.githubusercontent.com/${REPO}/main/install.ps1" >&2
      exit 1
      ;;
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

# Resilient download: resume partial transfers, retry CDN blips, show progress.
# No hard --max-time on the whole file — that killed slow Mac networks mid-download.
curl_download() {
  local url="$1"
  local dest="$2"
  curl -fL --progress-bar \
    --connect-timeout 30 \
    --retry 8 \
    --retry-all-errors \
    --retry-max-time 600 \
    --continue-at - \
    -o "$dest" \
    "$url"
}

download_release_binary() {
  need_cmd curl
  need_cmd tar
  local target archive url tmp ver resolved_tag checksums expected actual
  target="$(detect_target)"
  archive="salad-${target}.tar.gz"
  resolved_tag="$(resolve_release_tag)"
  local base_url="${SALAD_TERMINAL_BASE_URL:-https://github.com/${REPO}/releases/download/${resolved_tag}}"
  url="${base_url}/${archive}"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/salad-terminal.XXXXXX")"
  SALAD_INSTALL_TMP_DIR="$tmp"
  trap cleanup_install_tmp_dir EXIT

  echo "Downloading Salad Terminal (${resolved_tag} / ${target})…"
  if ! curl_download "$url" "${tmp}/${archive}"; then
    echo "error: could not download ${url}" >&2
    echo >&2
    echo "Check your network and retry. If it keeps failing:" >&2
    echo "  # optional: build from source" >&2
    echo "  brew install go git" >&2
    echo "  SALAD_FORCE_SOURCE=1 curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | bash" >&2
    exit 1
  fi
  if ! curl_download "${base_url}/SHA256SUMS" "${tmp}/SHA256SUMS"; then
    if [[ "${SALAD_ALLOW_UNVERIFIED_INSTALL:-}" != "1" ]]; then
      echo "error: release has no SHA256SUMS manifest; refusing unverified install" >&2
      exit 1
    fi
    echo "warning: installing without a checksum manifest because SALAD_ALLOW_UNVERIFIED_INSTALL=1" >&2
  else
    checksums="$(awk -v name="$archive" '$2 == name { print $1; exit }' "${tmp}/SHA256SUMS")"
    if [[ -z "$checksums" ]]; then
      echo "error: SHA256SUMS does not contain ${archive}" >&2
      exit 1
    fi
    if command -v sha256sum >/dev/null 2>&1; then
      actual="$(sha256sum "${tmp}/${archive}" | awk '{print $1}')"
    else
      need_cmd shasum
      actual="$(shasum -a 256 "${tmp}/${archive}" | awk '{print $1}')"
    fi
    if [[ "$actual" != "$checksums" ]]; then
      echo "error: checksum verification failed for ${archive}" >&2
      exit 1
    fi
  fi

  tar -xzf "${tmp}/${archive}" -C "$tmp"
  if [[ ! -f "${tmp}/salad" ]]; then
    echo "error: archive missing salad binary" >&2
    exit 1
  fi
  chmod +x "${tmp}/salad"

  ver="$(curl -fsSL --connect-timeout 10 --retry 3 --retry-all-errors "${base_url}/VERSION" 2>/dev/null || true)"
  if [[ -z "$ver" ]]; then
    ver="$resolved_tag"
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
  SALAD_INSTALL_TMP_DIR="$tmp"
  trap cleanup_install_tmp_dir EXIT
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
  build_from_dir "$SCRIPT_DIR"
else
  download_release_binary
fi

echo
echo "Installed. Run: salad"

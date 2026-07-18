#!/usr/bin/env bash
# One-shot install: put `salad` on your PATH.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

echo "Building Salad Terminal…"
go build -o salad ./cmd/salad

BIN_DIR="${SALAD_BIN_DIR:-}"
if [[ -z "$BIN_DIR" ]]; then
  if [[ -w /usr/local/bin ]]; then
    BIN_DIR=/usr/local/bin
  else
    BIN_DIR="${HOME}/.local/bin"
    mkdir -p "$BIN_DIR"
  fi
fi

install -m 755 salad "${BIN_DIR}/salad"
echo "Installed: ${BIN_DIR}/salad"

case ":$PATH:" in
  *":${BIN_DIR}:"*) ;;
  *)
    echo
    echo "Add to your shell profile (then reopen the terminal):"
    echo "  export PATH=\"${BIN_DIR}:\$PATH\""
    ;;
esac

echo
echo "Done. Next:"
echo "  salad login"
echo "  salad"

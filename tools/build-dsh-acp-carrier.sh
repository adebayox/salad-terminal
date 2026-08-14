#!/usr/bin/env bash
set -euo pipefail

# Build the opt-in Salad ACP carrier from a pinned DeepSeek Harness checkout.
# The checkout is deliberately an input: Salad does not vendor Cordis or let
# arbitrary plugins load inside the normal Salad chat process.

source_dir="${1:-}"
target="${2:-}"
output_dir="${3:-$PWD/dist-harness}"
pnpm_bin="${PNPM_BIN:-pnpm}"

if [[ -z "$source_dir" || ! -d "$source_dir" ]]; then
  echo "usage: $0 <deepseek-harness-checkout> [node24-linux-x64|node24-linux-arm64|node24-macos-x64|node24-macos-arm64] [output-dir]" >&2
  exit 2
fi

if [[ -z "$target" ]]; then
  case "$(uname -s):$(uname -m)" in
    Darwin:arm64) target="node24-macos-arm64" ;;
    Darwin:x86_64) target="node24-macos-x64" ;;
    Linux:aarch64|Linux:arm64) target="node24-linux-arm64" ;;
    Linux:x86_64) target="node24-linux-x64" ;;
    *) echo "unsupported host; pass an explicit DeepSeek target" >&2; exit 2 ;;
  esac
fi

case "$target" in
  node24-linux-x64|node24-linux-arm64|node24-macos-x64|node24-macos-arm64) ;;
  *) echo "unsupported target: $target" >&2; exit 2 ;;
esac

source_dir="$(cd "$source_dir" && pwd -P)"
mkdir -p "$output_dir"

node - "$source_dir" "$target" <<'NODE'
const fs = require('node:fs')
const path = require('node:path')

const root = process.argv[2]
const target = process.argv[3]
const packagePath = path.join(root, 'python/sdk-runtime/package.json')
const buildPath = path.join(root, 'scripts/build-exe-for-python-sdk.ts')
const workspacePath = path.join(root, 'pnpm-workspace.yaml')
let workspace = fs.readFileSync(workspacePath, 'utf8')
if (!workspace.includes('supportedArchitectures:')) {
  workspace += '\n# Salad carrier builds include native modules for the selected target.\nsupportedArchitectures:\n  os: [darwin, linux]\n  cpu: [x64, arm64]\n'
  fs.writeFileSync(workspacePath, workspace)
}
const packageJSON = JSON.parse(fs.readFileSync(packagePath, 'utf8'))
const dependencies = packageJSON.dependencies ?? (packageJSON.dependencies = {})
for (const name of [
  '@deepseek-ai/dsh-acp-demo',
  '@deepseek-ai/dsh-bash-sandbox',
  '@deepseek-ai/dsh-tool-subagent-report',
  '@deepseek-ai/dsh-tool-ralph',
]) {
  dependencies[name] ??= 'workspace:^'
}
const [, platform, arch] = target.split('-')
const nativePackage = platform === 'macos' ? `@koromix/koffi-darwin-${arch}` : `@koromix/koffi-linux-${arch}`
// pnpm omits optional native dependencies for the host architecture. Make
// the selected target's Koffi module explicit so cross-built carriers retain
// the sandbox FFI they need at runtime.
dependencies[nativePackage] = '3.1.4'
fs.writeFileSync(packagePath, `${JSON.stringify(packageJSON, null, 2)}\n`)

let build = fs.readFileSync(buildPath, 'utf8')
build = build.replace(/const ENTRY_BIN = '[^']+'/,
  "const ENTRY_BIN = 'node_modules/@deepseek-ai/dsh-acp-demo/lib/bin.js'")
build = build.replace(/const OUTPUT_BASENAME = '[^']+'/,
  "const OUTPUT_BASENAME = 'dsh-acp-agent-pkg'")
if (!build.includes("node_modules/@deepseek-ai/dsh-acp-demo/lib/bin.js") ||
    !build.includes("const OUTPUT_BASENAME = 'dsh-acp-agent-pkg'")) {
  throw new Error('DeepSeek build script shape changed; review the carrier patch before building')
}
fs.writeFileSync(buildPath, build)
NODE

cd "$source_dir"
"$pnpm_bin" install --no-frozen-lockfile
if [[ "${DSH_SKIP_BUILD:-0}" != "1" ]]; then
  "$pnpm_bin" run build
fi
"$pnpm_bin" exec tsx scripts/build-exe-for-python-sdk.ts --skip-build --targets="$target"

platform="${target#node24-}"
target_platform="${platform%-*}"
target_arch="${platform##*-}"
carrier="$(find dist-exe -maxdepth 1 -type f -name "dsh-acp-agent-pkg-${platform}" -print -quit)"
if [[ -z "$carrier" ]]; then
  echo "DeepSeek carrier was not produced for $target" >&2
  exit 1
fi

release_platform="$target_platform"
if [[ "$release_platform" == "macos" ]]; then
  # DeepSeek uses "macos" in its pkg target; Salad release artifacts use
  # the same darwin label as the terminal binary and installer.
  release_platform="darwin"
fi
release_arch="$target_arch"
if [[ "$release_arch" == "x64" ]]; then
  release_arch="amd64"
fi
install -m 700 "$carrier" "$output_dir/dsh-acp-agent-$release_platform-$release_arch"
install -m 600 examples/acp-agent/cordis.yml "$output_dir/cordis.yml"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$output_dir" && sha256sum "dsh-acp-agent-$release_platform-$release_arch" > SHA256SUMS)
else
  (cd "$output_dir" && shasum -a 256 "dsh-acp-agent-$release_platform-$release_arch" > SHA256SUMS)
fi
echo "Built $output_dir/dsh-acp-agent-$release_platform-$release_arch"

#!/usr/bin/env bash
set -euo pipefail

# Build the opt-in Salad ACP carrier from a pinned DeepSeek Harness checkout.
# The checkout is deliberately an input: Salad does not vendor Cordis or let
# arbitrary plugins load inside the normal Salad chat process.

source_dir="${1:-}"
target="${2:-}"
output_dir="${3:-$PWD/dist-harness}"
pnpm_bin="${PNPM_BIN:-pnpm}"
node_bin="${NODE_BIN:-node}"
default_dsh_source_revision="47f943859bef60e4160492346772ded9b24f765a"

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

node_major="$($node_bin -p "process.versions.node.split('.')[0]" 2>/dev/null || true)"
if [[ "$node_major" != "24" ]]; then
  echo "DeepSeek carrier builds require Node.js 24; found ${node_major:-unavailable}. Set NODE_BIN to a Node 24 executable." >&2
  exit 1
fi
node_dir="$(cd "$(dirname "$node_bin")" && pwd -P)"
PATH="$node_dir:$PATH"
export PATH

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
source_revision="$(git -C "$source_dir" rev-parse HEAD 2>/dev/null || true)"
if [[ -z "$source_revision" ]]; then
  echo "DeepSeek source must be a git checkout with a pinned revision" >&2
  exit 1
fi
expected_source_revision="${DSH_SOURCE_REVISION:-$default_dsh_source_revision}"
if [[ "$source_revision" != "$expected_source_revision" ]]; then
	echo "DeepSeek source revision $source_revision does not match expected pinned revision $expected_source_revision" >&2
  exit 1
fi
patch_sha256="$(shasum -a 256 "$script_dir/patch-dsh-security.py" | awk '{print $1}')"
# Apply Salad's fail-closed safety boundary to the pinned DSH source before
# compiling the carrier. The patch is shape-checked and aborts if upstream
# changes the seam we rely on.
python3 "$script_dir/patch-dsh-security.py" "$source_dir"

"$node_bin" - "$source_dir" "$target" <<'NODE'
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
  '@deepseek-ai/dsh-jobs',
  '@deepseek-ai/dsh-jobs-local',
  '@deepseek-ai/dsh-terminal',
  '@deepseek-ai/dsh-terminal-bash',
  '@deepseek-ai/dsh-tool-jobs',
  '@deepseek-ai/dsh-tool-subagent-report',
  '@deepseek-ai/dsh-tool-ralph',
  '@deepseek-ai/dsh-tool-terminal',
]) {
  dependencies[name] ??= 'workspace:^'
}
const [, platform, arch] = target.split('-')
const nativePackage = platform === 'macos' ? `@koromix/koffi-darwin-${arch}` : `@koromix/koffi-linux-${arch}`
// pnpm omits optional native dependencies for the host architecture. Make
// the selected target's Koffi module explicit so cross-built carriers retain
// the sandbox FFI they need at runtime. Keep this equal to the pinned
// lockfile version so the release cannot introduce an unreviewed native package.
dependencies[nativePackage] = '3.1.1'
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
# The carrier build starts from the pinned source and lockfile. The script adds
# only the target-specific workspace/native entries immediately above this
# command, so pnpm must be allowed to update those entries. Prefer the runner's
# cache, but allow a clean release runner to fetch the already-pinned graph.
"$pnpm_bin" install --prefer-offline --no-frozen-lockfile
lock_sha256="$(shasum -a 256 pnpm-lock.yaml | awk '{print $1}')"
node_version="$($node_bin --version)"
pnpm_version="$($pnpm_bin --version)"
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
if [[ "$target_platform" == "macos" ]]; then
  spawn_helper="$(find dist-exe -maxdepth 1 -type f -name "dsh-acp-agent-pkg-${platform}-spawn-helper" -print -quit)"
  if [[ -z "$spawn_helper" ]]; then
    echo "DeepSeek macOS carrier is missing its node-pty spawn helper for $target" >&2
    exit 1
  fi
  install -m 700 "$spawn_helper" "$output_dir/dsh-acp-agent-$release_platform-$release_arch-spawn-helper"
fi
install -m 600 examples/acp-agent/cordis.yml "$output_dir/cordis.yml"
"$node_bin" - "$output_dir" "$source_revision" "$patch_sha256" "$target" "$lock_sha256" "$node_version" "$pnpm_version" <<'NODE'
const fs = require('node:fs')
const path = require('node:path')
const [, , outputDir, sourceRevision, patchSHA256, target, lockSHA256, nodeVersion, pnpmVersion] = process.argv
fs.writeFileSync(path.join(outputDir, 'BUILD-METADATA'), [
  `DSH_SOURCE_REVISION=${sourceRevision}`,
  `SALAD_PATCH_SHA256=${patchSHA256}`,
  `PNPM_LOCK_SHA256=${lockSHA256}`,
  `TARGET=${target}`,
  `NODE_VERSION=${nodeVersion}`,
  `PNPM_VERSION=${pnpmVersion}`,
  'ROLLBACK=retain the previous managed carrier until the new checksum and ACP smoke pass',
  '',
].join('\n'), { mode: 0o600 })
NODE
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$output_dir" && sha256sum dsh-acp-agent-* cordis.yml BUILD-METADATA > SHA256SUMS)
else
  (cd "$output_dir" && shasum -a 256 dsh-acp-agent-* cordis.yml BUILD-METADATA > SHA256SUMS)
fi
echo "Built $output_dir/dsh-acp-agent-$release_platform-$release_arch"

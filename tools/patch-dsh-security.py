#!/usr/bin/env python3
"""Apply Salad's fail-closed policy to the pinned DeepSeek Harness source."""

from pathlib import Path
import sys

root = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else None
if root is None or not root.exists():
    raise SystemExit("usage: patch-dsh-security.py <deepseek-harness-checkout>")


def replace_once(file: Path, before: str, after: str, label: str) -> None:
    source = file.read_text()
    count = source.count(before)
    if count != 1:
        raise RuntimeError(f"DeepSeek source shape changed while patching {label}: found {count} matches")
    file.write_text(source.replace(before, after))


profile = root / "packages/sandbox/sandbox-local/src/profiles.ts"
replace_once(
    profile,
    "  const args = ['--ro-bind', '/', '/', '--dev', '/dev', '--proc', '/proc', '--die-with-parent']",
    """  const args = ['--ro-bind', '/', '/', '--dev', '/dev', '--proc', '/proc', '--die-with-parent']
  // This profile wraps model-controlled shell processes. The DSH provider
  // remains in the parent process and keeps its loopback bridge.
  if (process.env.DSH_NETWORK_MODE !== 'allow') args.push('--unshare-net')""",
    "Linux shell network isolation",
)
replace_once(
    profile,
    "  return landlockGrantArgs({ readOnly: ['/'], readWrite })",
    """  if (process.env.DSH_NETWORK_MODE !== 'allow') {
    throw new Error('DSH network-deny policy requires bubblewrap on Linux; Landlock cannot enforce network isolation')
  }
  return landlockGrantArgs({ readOnly: ['/'], readWrite })""",
    "Linux Landlock fail-closed network policy",
)
replace_once(
    profile,
    "  const forms = ['(version 1)', '(allow default)', '(deny file-write*)', `(allow file-write* (literal ${sbplString('/dev/null')}))`]",
    """  const forms = ['(version 1)', '(allow default)', '(deny file-write*)']
  if (process.env.DSH_NETWORK_MODE !== 'allow') forms.push('(deny network*)')
  const rootPattern = policy.workspaceRoot.replaceAll('\\\\', '\\\\\\\\').replaceAll('.', '\\\\.')
  const secretPattern = String.raw`^${rootPattern}/(?:[^/]+/)*(?:[.]env(?:[.][^/]*)?|[.]npmrc|[.]pypirc|[.]netrc|credentials(?:[.][^/]*)?|id_(?:rsa|dsa|ecdsa|ed25519)|[^/]+[.](?:pem|key|p12|pfx))(?:/|$)`
  forms.push(`(deny file-read* (regex #"${secretPattern}") (with no-log))`)
  forms.push(`(allow file-write* (literal ${sbplString('/dev/null')}))`)""",
    "macOS Seatbelt network and sensitive-file policy",
)

fs_sandbox = root / "packages/fs/fs-sandbox/src/index.ts"
replace_once(
    fs_sandbox,
    "import { Context } from '@deepseek-ai/cordis'",
    """import { Context } from '@deepseek-ai/cordis'
import { basename } from 'node:path'""",
    "filesystem sensitive-path import",
)
replace_once(
    fs_sandbox,
    "export type Config = LocalConfig",
    """export type Config = LocalConfig

// Credential-shaped files are not readable by model-controlled DSH tools.
const SENSITIVE_NAMES = new Set(['.npmrc', '.pypirc', '.netrc', 'credentials', 'id_rsa', 'id_dsa', 'id_ecdsa', 'id_ed25519'])

function isSensitivePath(target: FsTarget): boolean {
  const candidate = String(target.targetKey || target.displayPath).replaceAll('\\\\', '/')
  const base = basename(candidate).toLowerCase()
  const parts = candidate.split('/').filter(Boolean)
  return base.startsWith('.env')
    || SENSITIVE_NAMES.has(base)
    || base.endsWith('.pem')
    || base.endsWith('.key')
    || base.endsWith('.p12')
    || base.endsWith('.pfx')
    || parts.some(part => part === '.ssh' || part === '.aws' || part === '.azure')
}

function denySensitiveRead(target: FsTarget): void {
  if (isSensitivePath(target)) {
    throw new FsError(
      `cannot read "${target.displayPath}": credential-shaped files are protected in Salad Terminal`,
      'FS_SANDBOX_DENIED',
    )
  }
}""",
    "filesystem sensitive-path policy",
)
replace_once(
    fs_sandbox,
    """  /**
   * Fence the write by the per-call policy, then delegate to the inherited
""",
    """  override async readText(target: FsTarget, signal?: AbortSignal): Promise<string> {
    denySensitiveRead(target)
    return super.readText(target, signal)
  }

  override async streamText(target: FsTarget, signal?: AbortSignal): Promise<AsyncIterable<string>> {
    denySensitiveRead(target)
    return super.streamText(target, signal)
  }

  override async readBytes(target: FsTarget, signal: AbortSignal | undefined, maxBytes: number): Promise<Uint8Array> {
    denySensitiveRead(target)
    return super.readBytes(target, signal, maxBytes)
  }

  /**
   * Fence the write by the per-call policy, then delegate to the inherited
""",
    "filesystem sensitive-path read methods",
)
replace_once(
    fs_sandbox,
    "    if (mode === 'danger-full-access') return target",
    """    if (isSensitivePath(target)) {
      throw new FsError(
        `cannot modify "${target.displayPath}": credential-shaped files are protected in Salad Terminal`,
        'FS_SANDBOX_DENIED',
      )
    }
    if (mode === 'danger-full-access') return target""",
    "filesystem sensitive-path write policy",
)

sandbox_local = root / "packages/sandbox/sandbox-local/src/index.ts"
replace_once(
    sandbox_local,
    """  confine(argv: readonly string[], policy: SandboxPolicy): ConfinedArgv {
    if (this.runnerCommand !== undefined) {""",
    """  confine(argv: readonly string[], policy: SandboxPolicy): ConfinedArgv {
    if (process.platform === 'win32' && process.env.DSH_NETWORK_MODE !== 'allow') {
      throw new SandboxUnavailableError(policy.mode, 'DSH network-deny policy is not available on the Windows ACL runner')
    }
    if (this.runnerCommand !== undefined) {""",
    "Windows network-policy fail closed",
)

print("Applied Salad Terminal security policy to pinned DeepSeek Harness source")

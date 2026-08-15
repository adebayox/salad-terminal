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
  // The shell can address absolute paths, so protect credential-shaped
  // locations globally rather than only below the trusted workspace.
  // Keep this as simple alternatives: Seatbelt's regex parser treats some
  // nested optional groups differently from the regex engines used in JS.
  const secretPattern = String.raw`/[.]env|/[.]ssh|/[.]aws|/[.]azure|/[.]npmrc|/[.]pypirc|/[.]netrc|/credentials|/id_(rsa|dsa|ecdsa|ed25519)|/[^/]+[.](pem|key|p12|pfx)`
  forms.push(`(deny file-read* (regex #"${secretPattern}") (with no-log))`)
  forms.push(`(allow file-write* (literal ${sbplString('/dev/null')}))`)""",
    "macOS Seatbelt network and sensitive-file policy",
)

acp_server = root / "packages/acp/acp/src/index.ts"
replace_once(
    acp_server,
    """  type NewSessionRequest,
  type NewSessionResponse,
  type PromptRequest,""",
    """  type NewSessionRequest,
  type NewSessionResponse,
  type ResumeSessionRequest,
  type ResumeSessionResponse,
  type CloseSessionRequest,
  type CloseSessionResponse,
  type PromptRequest,""",
    "ACP session restore imports",
)
replace_once(
    acp_server,
    """  const sessions = new Map<SessionId, SessionRecord>()
  let closed = false""",
    """  const sessions = new Map<SessionId, SessionRecord>()
  // The example ACP composition includes a durable persistence backend. Keep
  // the capability honest for custom compositions that omit it.
  const persistenceAvailable = ctx.get('sessionPersistence') !== undefined
  let closed = false""",
    "ACP persistence capability detection",
)
replace_once(
    acp_server,
    """          agentCapabilities: {
            promptCapabilities: { image: false, audio: false, embeddedContext: false },
          },""",
    """          agentCapabilities: {
            promptCapabilities: { image: false, audio: false, embeddedContext: false },
            sessionCapabilities: {
              ...(persistenceAvailable ? { resume: {} } : {}),
              close: {},
            },
          },""",
    "ACP session restore capability advertisement",
)
replace_once(
    acp_server,
    """        return { sessionId }
      },

      async prompt(params: PromptRequest): Promise<PromptResponse> {""",
    """        return { sessionId }
      },

      async resumeSession(params: ResumeSessionRequest): Promise<ResumeSessionResponse> {
        assertOpen()
        if (!persistenceAvailable) throw invalidParams('session resume is unavailable in this composition')
        if (!isAbsolute(params.cwd)) throw invalidParams(`cwd must be an absolute path: ${params.cwd}`)
        if (params.additionalDirectories !== undefined && params.additionalDirectories.length > 0) {
          throw invalidParams('additionalDirectories is not supported')
        }
        if (params.mcpServers !== undefined && params.mcpServers.length > 0) throw invalidParams('mcpServers is not supported')
        const sessionId = SessionId(params.sessionId)
        if (sessions.has(sessionId)) throw invalidParams(`session is already active: ${params.sessionId}`)
        const handle = await agents.resume({
          resumeSessionId: sessionId,
          agentOptions: agentOptions(config),
        })
        /* v8 ignore next 4 -- a real stdio close can race an in-flight create. */
        if (closed) {
          await handle.dispose()
          throw internalError('connection closed during session/resume')
        }
        if (handle.agent.session.header.cwd !== params.cwd) {
          await handle.dispose()
          throw invalidParams(`session cwd does not match: ${String(handle.agent.session.header.cwd)}`)
        }
        sessions.set(sessionId, {
          agent: handle.agent,
          dispose: () => handle.dispose(),
          inflight: undefined,
        })
        return {}
      },

      async closeSession(params: CloseSessionRequest): Promise<CloseSessionResponse> {
        const sessionId = SessionId(params.sessionId)
        const record = sessions.get(sessionId)
        if (record === undefined) return {}
        sessions.delete(sessionId)
        record.agent.cancel({ kind: 'user' })
        settlePrompt(record, 'cancelled')
        await record.dispose()
        return {}
      },

      async prompt(params: PromptRequest): Promise<PromptResponse> {""",
    "ACP session restore and close handlers",
)

jsonrpc_server = root / "packages/sdk/server/src/server.ts"
replace_once(
    jsonrpc_server,
    """    const handle = await this.ctx.agents.create({
      sessionId: SessionId(sessionId),
      meta: { cwd: this.cwd },
      agentOptions: {
        provider: this.provider,
        model: this.model,
        ...this.maxTokens === undefined ? {} : { maxTokens: this.maxTokens },
      },
    })""",
    """    const persistence = this.ctx.get('sessionPersistence')
    if (persistence !== undefined && (await persistence.list()).some((header: { id: string }) => header.id === sessionId)) {
      const handle = await this.ctx.agents.resume({
        resumeSessionId: SessionId(sessionId),
        agentOptions: {
          provider: this.provider,
          model: this.model,
          ...this.maxTokens === undefined ? {} : { maxTokens: this.maxTokens },
        },
      })
      return { handle }
    }
    const handle = await this.ctx.agents.create({
      sessionId: SessionId(sessionId),
      meta: { cwd: this.cwd },
      agentOptions: {
        provider: this.provider,
        model: this.model,
        ...this.maxTokens === undefined ? {} : { maxTokens: this.maxTokens },
      },
    })""",
    "JSON-RPC persisted-session resume",
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

# The stock ACP example exposes one-shot bash only. Salad's engineer path also
# needs the official DSH persistent PTY and background-job capabilities so a
# developer can start a dev server, inspect it, and stop it without inventing
# a second process manager in the Salad CLI.
acp_config = root / "examples/acp-agent/cordis.yml"
replace_once(
    acp_config,
    """- id: bash
  name: '@deepseek-ai/dsh-bash-sandbox'
  config:
    timeoutMs: 60000

- id: approval""",
    """- id: bash
  name: '@deepseek-ai/dsh-bash-sandbox'
  config:
    timeoutMs: 60000

# Persistent terminal sessions own interactive processes and their cleanup.
- id: pty
  name: '@deepseek-ai/dsh-terminal'

- id: terminal-bash
  name: '@deepseek-ai/dsh-terminal-bash'
  config:
    pollIntervalMs: 10
    exactProbeAfterMs: 20
    idleSilenceMs: 250
    handoffGraceMs: 250
    timeoutMs: 2000
    disposeGraceMs: 500

# Background jobs make long-running commands observable and cancellable.
- id: jobs
  name: '@deepseek-ai/dsh-jobs'

- id: jobs-local
  name: '@deepseek-ai/dsh-jobs-local'

- id: approval""",
    "persistent terminal and job backends",
)
replace_once(
    acp_config,
    """- id: tool-fs
  name: '@deepseek-ai/dsh-tool-fs'

# `configPath` is read once""",
    """- id: tool-fs
  name: '@deepseek-ai/dsh-tool-fs'

- id: tool-jobs
  name: '@deepseek-ai/dsh-tool-jobs'
  config:
    completionDelivery: quiet

- id: tool-terminal
  name: '@deepseek-ai/dsh-tool-terminal'

# `configPath` is read once""",
    "persistent terminal and job tools",
)

print("Applied Salad Terminal security policy to pinned DeepSeek Harness source")

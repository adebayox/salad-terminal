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
  const networkMode = process.env.DSH_NETWORK_MODE ?? 'deny'
  if (networkMode !== 'allow') args.push('--unshare-net')""",
    "Linux shell network isolation",
)
replace_once(
    profile,
    "  return landlockGrantArgs({ readOnly: ['/'], readWrite })",
    """  const networkMode = process.env.DSH_NETWORK_MODE ?? 'deny'
  if (networkMode !== 'allow') {
    throw new Error('DSH network-deny policy requires bubblewrap on Linux; Landlock cannot enforce network isolation')
  }
  return landlockGrantArgs({ readOnly: ['/'], readWrite })""",
    "Linux Landlock fail-closed network policy",
)
replace_once(
    profile,
    "  const forms = ['(version 1)', '(allow default)', '(deny file-write*)', `(allow file-write* (literal ${sbplString('/dev/null')}))`]",
    """  const forms = ['(version 1)', '(allow default)', '(deny file-write*)']
  const networkMode = process.env.DSH_NETWORK_MODE ?? 'deny'
  if (networkMode !== 'allow') forms.push('(deny network*)')
  if (networkMode === 'loopback') {
    // Local development servers need a narrow exception without granting the
    // model internet access.
    forms.push('(allow network-bind (local ip "localhost:*"))')
    forms.push('(allow network-inbound (local ip "localhost:*"))')
    forms.push('(allow network-outbound (remote ip "localhost:*"))')
  }
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
    """  // Emit only committed assistant text. Raw chunks, reasoning, tools, plans,
  // titles, and retry markers are presentation or trace data and stay off the
  // automation wire.
  ctx.on('session/event', (session, event: SessionEvent) => {
    const record = sessions.get(session.header.id)
    if (record === undefined || record.agent.session !== session) return
    try {
      if (event.type === 'assistant/message') {
        for (const block of event.data.message.content) {
          if (block.type === 'text' && block.text.length > 0) {
            notify({
              sessionId: record.agent.session.id,
              update: {
                sessionUpdate: 'agent_message_chunk',
                content: { type: 'text', text: block.text },
              },
            })
          } else if (block.type === 'image') {
            notify({
              sessionId: record.agent.session.id,
              update: {
                sessionUpdate: 'agent_message_chunk',
                content: {
                  type: 'text',
                  text: `[image attachment ${block.attachment.attachmentId}]`,
                },
              },
            })
          }
        }
      }
    } finally {
      const inflight = record.inflight
      if (inflight !== undefined && event.type === 'turn/end' && inflight.turn === event.data.turn) {
        if (event.data.reason.kind === 'error') {
          // Model failures surface immediately as prompt errors; ordinary
          // endings wait for whole-agent idle below.
          record.inflight = undefined
          rejectFromError(inflight, event.data.reason)
        } else {
          inflight.endReason = event.data.reason
        }
      }
    }
  })""",
    """  const presentationCalls = new Map<SessionId, Map<string, {
    name: string
    rawInput: unknown
    locations: Array<{ path: string }>
  }>>()
  const presentationLimit = 12_000
  const redactPresentationText = (value: string): string => value
    .replace(/(api[_-]?key|access[_-]?token|refresh[_-]?token|password|secret|authorization)\\s*[:=]\\s*[\"']?[^,\\s\"'}]+/gi, '$1=[redacted]')
    .replace(/\\bsk-[A-Za-z0-9_-]{8,}\\b/g, '[redacted]')
    .slice(0, presentationLimit)

  const safeToolInput = (raw: string): { value: unknown; locations: Array<{ path: string }> } => {
    let parsed: unknown
    try {
      parsed = JSON.parse(raw)
    } catch {
      return { value: {}, locations: [] }
    }
    if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return { value: {}, locations: [] }
    }
    const input = parsed as Record<string, unknown>
    const safe: Record<string, unknown> = {}
    const locations: Array<{ path: string }> = []
    for (const key of ['path', 'file', 'cwd', 'command', 'query', 'pattern', 'glob']) {
      const candidate = input[key]
      if (typeof candidate !== 'string' || candidate.length === 0) continue
      const value = redactPresentationText(candidate)
      safe[key] = value
      if ((key === 'path' || key === 'file') && value.length > 0) locations.push({ path: value })
    }
    return { value: safe, locations }
  }

  const toolKind = (name: string): string => {
    const lower = name.toLowerCase()
    if (/(read|list|stat|search|grep|find)/.test(lower)) return 'read'
    if (/(write|edit|patch|create|apply)/.test(lower)) return 'edit'
    if (/(delete|remove|unlink)/.test(lower)) return 'delete'
    if (/(bash|shell|exec|command|terminal|job)/.test(lower)) return 'execute'
    if (/(fetch|http|web)/.test(lower)) return 'fetch'
    return 'other'
  }

  const resultText = (content: unknown): string => {
    if (!Array.isArray(content)) return ''
    return content
      .filter((block): block is { type: string; text: string } =>
        typeof block === 'object' && block !== null && (block as { type?: unknown }).type === 'text' && typeof (block as { text?: unknown }).text === 'string')
      .map(block => redactPresentationText(block.text))
      .join('\\n')
      .slice(0, presentationLimit)
  }

  const emitToolCall = (sessionId: SessionId, data: { callId: string; name: string; arguments: string }): void => {
    const parsed = safeToolInput(data.arguments)
    let calls = presentationCalls.get(sessionId)
    if (calls === undefined) {
      calls = new Map()
      presentationCalls.set(sessionId, calls)
    }
    calls.set(data.callId, { name: data.name, rawInput: parsed.value, locations: parsed.locations })
    notify({
      sessionId,
      update: {
        sessionUpdate: 'tool_call',
        toolCallId: data.callId,
        title: data.name,
        kind: toolKind(data.name),
        status: 'pending',
        rawInput: parsed.value,
        ...(parsed.locations.length > 0 ? { locations: parsed.locations } : {}),
      },
    } as unknown as SessionNotification)
  }

  const emitToolResult = (sessionId: SessionId, data: { callId: string; content: unknown; failed: boolean }): void => {
    const call = presentationCalls.get(sessionId)?.get(data.callId)
    const text = resultText(data.content)
    notify({
      sessionId,
      update: {
        sessionUpdate: 'tool_call_update',
        toolCallId: data.callId,
        status: data.failed ? 'failed' : 'completed',
        ...(call !== undefined && call.locations.length > 0 ? { locations: call.locations } : {}),
        ...(text.length > 0 ? {
          content: [{ type: 'content', content: { type: 'text', text } }],
          rawOutput: { text },
        } : {}),
      },
    } as unknown as SessionNotification)
    presentationCalls.get(sessionId)?.delete(data.callId)
  }

  const emitPlan = (sessionId: SessionId, todos: readonly { content: string; status: string }[]): void => {
    notify({
      sessionId,
      update: {
        sessionUpdate: 'plan',
        entries: todos.map(todo => ({
          content: redactPresentationText(todo.content),
          status: todo.status === 'completed' ? 'completed' : todo.status === 'in_progress' ? 'in_progress' : 'pending',
          priority: 'medium',
        })),
      },
    } as unknown as SessionNotification)
  }

  // The stock ACP bridge deliberately exposes only committed assistant text.
  // Salad's engineer client needs standard ACP activity updates as well, so
  // this carrier emits bounded, redacted tool and plan presentations while
  // keeping the DSH session log as the source of truth.
  ctx.on('session/event', (session, event: SessionEvent) => {
    const record = sessions.get(session.header.id)
    if (record === undefined || record.agent.session !== session) return
    try {
      if (event.type === 'tool/call') {
        emitToolCall(record.agent.session.id, {
          callId: String(event.data.callId),
          name: event.data.name,
          arguments: event.data.arguments,
        })
      } else if (event.type === 'tool/result') {
        emitToolResult(record.agent.session.id, {
          callId: String(event.data.message.source.callId),
          content: event.data.message.content,
          failed: event.data.error !== undefined || event.data.message.content.some(block => block.type === 'tool-result' && block.isError === true),
        })
      } else if (event.type === 'todo/write') {
        emitPlan(record.agent.session.id, event.data.todos)
      } else if (event.type === 'assistant/message') {
        for (const block of event.data.message.content) {
          if (block.type === 'text' && block.text.length > 0) {
            notify({
              sessionId: record.agent.session.id,
              update: {
                sessionUpdate: 'agent_message_chunk',
                content: { type: 'text', text: redactPresentationText(block.text) },
              },
            })
          } else if (block.type === 'image') {
            notify({
              sessionId: record.agent.session.id,
              update: {
                sessionUpdate: 'agent_message_chunk',
                content: {
                  type: 'text',
                  text: `[image attachment ${block.attachment.attachmentId}]`,
                },
              },
            })
          }
        }
      }
    } finally {
      const inflight = record.inflight
      if (inflight !== undefined && event.type === 'turn/end' && inflight.turn === event.data.turn) {
        if (event.data.reason.kind === 'error') {
          // Model failures surface immediately as prompt errors; ordinary
          // endings wait for whole-agent idle below.
          record.inflight = undefined
          rejectFromError(inflight, event.data.reason)
        } else {
          inflight.endReason = event.data.reason
        }
      }
    }
  })""",
    "ACP rich activity event bridge",
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
        presentationCalls.delete(sessionId)
        record.agent.cancel({ kind: 'user' })
        settlePrompt(record, 'cancelled')
        await record.dispose()
        return {}
      },

      async prompt(params: PromptRequest): Promise<PromptResponse> {""",
    "ACP session restore and close handlers",
)
replace_once(
    acp_server,
    """    for (const record of records) {
      record.agent.cancel({ kind: 'user' })
      settlePrompt(record, 'cancelled')
    }""",
    """    for (const record of records) {
      presentationCalls.delete(record.agent.session.id)
      record.agent.cancel({ kind: 'user' })
      settlePrompt(record, 'cancelled')
    }""",
    "ACP presentation state cleanup",
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

# Background jobs make long-running commands observable and cancellable. The
# local package supplies the abstract @deepseek-ai/dsh-jobs registry.
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
replace_once(
    acp_config,
    """    persona: |
      You are a coding assistant powered by the {{model}} model. Your working directory is {{cwd}}. Your bash tool runs under a file sandbox — a `[sandbox: file access denied …]` result is policy, not a command bug.

      Verify your work by running the code or tests. Keep answers brief and factual.""",
    """    persona: |
      You are a coding assistant powered by the {{model}} model. Your working directory is {{cwd}}. Your bash tool runs under a file sandbox — a `[sandbox: file access denied …]` result is policy, not a command bug.

      Verify your work by running the code or tests. Keep answers brief and factual.

      Use the persistent terminal tools for anything that may keep running: dev servers, watchers, REPLs, interactive commands, and commands expected to last longer than a short check. Use terminal_open, terminal_read, terminal_signal, and terminal_close so the process has an observable owner and is cleaned up. Use job_list, job_output, and job_kill when a background job is appropriate. Do not use `&`, nohup, disown, or an untracked background process for a dev server. After starting a long-running process, report the exact command, the tool/job id, the listening address if relevant, and an external verification result.

      Use the ordinary bash tool for short, bounded commands such as focused tests, formatting, git inspection, and builds. When setting a command environment, use an explicit form such as `env PORT=4321 npm run start` or a command-line flag, then verify the actual listener with terminal output and a separate request. If a command reports permission denied while binding localhost, explain that the run needs the explicit Salad Terminal loopback mode (`salad harness --network loopback`) instead of retrying or claiming the server started. Never claim a command or verification succeeded without its tool output.""",
    "persistent terminal usage guidance",
)

replace_once(
    acp_config,
    """      Use the ordinary bash tool for short, bounded commands such as focused tests, formatting, git inspection, and builds. When setting a command environment, use an explicit form such as `env PORT=4321 npm run start` or a command-line flag, then verify the actual listener with terminal output and a separate request. If a command reports permission denied while binding localhost, explain that the run needs the explicit Salad Terminal loopback mode (`salad harness --network loopback`) instead of retrying or claiming the server started. Never claim a command or verification succeeded without its tool output. For a requested read-only review, use the bounded `subagent_fork` tool, wait for its returned result, and report the concrete finding. If it fails, times out, or returns no result, say so plainly and never invent reviewer approval. Do not retry the same failed edit indefinitely; after two unsuccessful attempts, stop and report the exact failure.""",
    "reviewer honesty and bounded repair guidance",
)

print("Applied Salad Terminal security policy to pinned DeepSeek Harness source")

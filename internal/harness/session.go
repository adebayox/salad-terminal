package harness

// session.go exposes the ACP process as a client-owned session. The existing
// one-shot helpers remain available for diagnostics and compatibility, while
// the Salad TUI uses this object to keep prompt entry, transcript rendering,
// approvals, and cancellation in one terminal surface.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SessionEvent is a UI-safe event emitted by an ACP session.
type SessionEvent struct {
	Kind         string
	Text         string
	SessionID    string
	PermissionID string
	ToolID       string
	ToolName     string
	ToolKind     string
	ToolStatus   string
	ToolInput    string
	ToolOutput   string
	Locations    []string
	Plan         []workspacePlanItem
	Err          error
}

type sessionCommand struct {
	kind   string
	text   string
	result chan error
}

type permissionResponse struct {
	id    string
	allow bool
}

// errPromptTimeout marks a prompt whose response stream can no longer be
// safely reused. The carrier may still emit a late response after the client
// deadline, so the persistent session is retired instead of allowing that
// response to be mistaken for the next prompt's result.
var errPromptTimeout = errors.New("ACP prompt timeout; session retired")

// Session owns one ACP child and one ACP session. Prompt calls are serialized,
// which matches ACP's turn model and prevents two terminal sends from being
// interleaved inside one agent session.
type Session struct {
	commands    chan sessionCommand
	permissions chan permissionResponse
	events      chan SessionEvent
	done        chan struct{}
	closeOnce   sync.Once
	stopMu      sync.Mutex
	stop        context.CancelFunc
}

// StartSession starts and initializes one ACP session in the trusted
// workspace. It returns only after initialize and session/new/session/load
// have completed, so the caller can render a ready state immediately.
func StartSession(ctx context.Context, opts Options) (*Session, error) {
	if strings.TrimSpace(opts.Cwd) == "" {
		return nil, errors.New("harness workspace is required")
	}
	if opts.Command == "" {
		opts.Command = firstNonEmpty(os.Getenv("SALAD_DSH_COMMAND"), defaultACPCommand)
	}
	if opts.Provider == "" {
		opts.Provider = firstNonEmpty(os.Getenv("SALAD_DSH_PROVIDER"), defaultProvider)
	}
	if opts.Model == "" {
		opts.Model = firstNonEmpty(os.Getenv("SALAD_DSH_MODEL"), defaultModel)
	}
	if opts.PromptTimeout <= 0 {
		opts.PromptTimeout = promptTimeoutFromEnv()
	}

	s := &Session{
		commands: make(chan sessionCommand),
		// The TUI receives a permission event before the ACP reader begins
		// waiting for the answer. Buffer one response so a fast key press is
		// never lost in that handoff window.
		permissions: make(chan permissionResponse, 1),
		events:      make(chan SessionEvent, 32),
		done:        make(chan struct{}),
	}
	ready := make(chan error, 1)
	go s.run(ctx, opts, ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return s, nil
}

// Events returns the stream of safe assistant/status/permission events.
func (s *Session) Events() <-chan SessionEvent { return s.events }

// Prompt queues one user turn. The returned error reports the protocol-level
// completion of that turn, not merely that the request was queued.
func (s *Session) Prompt(ctx context.Context, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("harness prompt cannot be empty")
	}
	result := make(chan error, 1)
	command := sessionCommand{kind: "prompt", text: text, result: result}
	select {
	case s.commands <- command:
	case <-s.done:
		return errors.New("DeepSeek Harness session is closed")
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-result:
		return err
	case <-s.done:
		return errors.New("DeepSeek Harness session is closed")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RespondPermission completes the permission request currently shown by the
// Salad TUI. Unknown request ids are ignored by the session loop.
func (s *Session) RespondPermission(id string, allow bool) {
	select {
	case s.permissions <- permissionResponse{id: id, allow: allow}:
	case <-s.done:
	default:
	}
}

// Close cancels the ACP session and waits briefly for the child to be reaped.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.stopMu.Lock()
		stop := s.stop
		s.stopMu.Unlock()
		if stop != nil {
			stop()
		}
		select {
		case <-s.done:
		case <-time.After(3 * time.Second):
		}
	})
}

func (s *Session) emit(event SessionEvent) {
	select {
	case s.events <- event:
	case <-s.done:
	}
}

func (s *Session) run(ctx context.Context, opts Options, ready chan<- error) {
	cmd := exec.Command(opts.Command, opts.Args...)
	prepareProcessGroup(cmd)
	cmd.Dir = opts.Cwd
	cmd.Env = scrubbedEnvironment(withHarnessSafetyDefaults(opts.Env, opts.Cwd))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		ready <- fmt.Errorf("start harness stdin: %w", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ready <- fmt.Errorf("start harness stdout: %w", err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		ready <- fmt.Errorf("start harness stderr: %w", err)
		return
	}
	if err := cmd.Start(); err != nil {
		ready <- fmt.Errorf("start DeepSeek Harness ACP (%s): %w", opts.Command, err)
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.stopMu.Lock()
	s.stop = cancel
	s.stopMu.Unlock()
	defer cancel()
	defer func() {
		s.stopMu.Lock()
		s.stop = nil
		s.stopMu.Unlock()
	}()
	defer close(s.done)
	defer close(s.events)
	processDone := make(chan error, 1)
	go func() { processDone <- cmd.Wait() }()
	defer func() {
		_ = stdin.Close()
		select {
		case <-processDone:
		case <-time.After(3 * time.Second):
			_ = killProcessTree(cmd)
			<-processDone
		}
	}()
	go func() { _, _ = io.Copy(io.Discard, stderr) }()

	var writeMu sync.Mutex
	writeFrame := func(frame any) error {
		payload, err := json.Marshal(frame)
		if err != nil {
			return fmt.Errorf("encode ACP request: %w", err)
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		if _, err := fmt.Fprintf(stdin, "%s\n", payload); err != nil {
			return fmt.Errorf("write ACP request: %w", err)
		}
		return nil
	}
	writeResponse := func(id rpcID, result any) error {
		payload, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("encode ACP response: %w", err)
		}
		frame := []byte(`{"jsonrpc":"2.0","id":`)
		frame = append(frame, rpcIDJSON(id)...)
		frame = append(frame, []byte(`,"result":`)...)
		frame = append(frame, payload...)
		frame = append(frame, '}', '\n')
		writeMu.Lock()
		defer writeMu.Unlock()
		if _, err := stdin.Write(frame); err != nil {
			return fmt.Errorf("write ACP response: %w", err)
		}
		return nil
	}

	type readResult struct {
		frame rpcFrame
		err   error
	}
	frames := make(chan readResult, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), maxFrameBytes)
		for scanner.Scan() {
			frame, decodeErr := decodeFrame(scanner.Bytes())
			select {
			case frames <- readResult{frame: frame, err: decodeErr}:
			case <-runCtx.Done():
				return
			}
			if decodeErr != nil {
				return
			}
		}
		var err error
		if scanErr := scanner.Err(); scanErr != nil {
			err = fmt.Errorf("read ACP response: %w", scanErr)
		} else if runCtx.Err() != nil {
			err = runCtx.Err()
		} else {
			err = errors.New("DeepSeek Harness ACP closed its output")
		}
		select {
		case frames <- readResult{err: err}:
		case <-runCtx.Done():
		}
	}()
	read := func(readCtx context.Context) (rpcFrame, error) {
		select {
		case value := <-frames:
			return value.frame, value.err
		case <-readCtx.Done():
			return rpcFrame{}, readCtx.Err()
		}
	}

	request := func(id, method string, params any) (json.RawMessage, error) {
		if err := writeFrame(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for {
			frame, err := read(runCtx)
			if err != nil {
				return nil, err
			}
			if frame.Method != "" {
				if err := s.handleFrame(runCtx, opts, frame, "", writeResponse); err != nil {
					return nil, err
				}
				continue
			}
			if string(frame.ID) != id {
				return nil, fmt.Errorf("unexpected ACP response id %q", frame.ID)
			}
			if frame.Error != nil {
				return nil, fmt.Errorf("ACP %s failed (%d): %s", method, frame.Error.Code, frame.Error.Message)
			}
			return frame.Result, nil
		}
	}

	initializedRaw, err := request("1", "initialize", acpInitializeParams(opts))
	if err != nil {
		ready <- err
		s.emit(SessionEvent{Kind: "error", Err: err})
		return
	}
	var initialized acpInitializeResult
	if err := json.Unmarshal(initializedRaw, &initialized); err != nil {
		ready <- fmt.Errorf("decode ACP initialize response: %w", err)
		return
	}
	requestedSession := strings.TrimSpace(opts.SessionID)
	canResume := len(initialized.AgentCapabilities.SessionCapabilities.Resume) > 0 && string(initialized.AgentCapabilities.SessionCapabilities.Resume) != "null"
	sessionID := ""
	if requestedSession != "" && (initialized.AgentCapabilities.LoadSession || canResume) {
		method := "session/resume"
		if initialized.AgentCapabilities.LoadSession {
			method = "session/load"
		}
		if _, err := request("2", method, map[string]any{"sessionId": requestedSession, "cwd": opts.Cwd, "mcpServers": []any{}}); err != nil {
			ready <- err
			return
		}
		sessionID = requestedSession
	} else {
		newRaw, err := request("2", "session/new", map[string]any{"cwd": opts.Cwd, "mcpServers": []any{}})
		if err != nil {
			ready <- err
			return
		}
		var session struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(newRaw, &session); err != nil || session.SessionID == "" {
			ready <- errors.New("ACP session/new returned no session id")
			return
		}
		sessionID = session.SessionID
	}
	if opts.OnSessionID != nil {
		opts.OnSessionID(sessionID)
	}
	s.emit(SessionEvent{Kind: "ready", SessionID: sessionID})
	ready <- nil

	promptID := 3
	for {
		select {
		case <-runCtx.Done():
			return
		case command := <-s.commands:
			switch command.kind {
			case "prompt":
				err := s.promptTurn(runCtx, opts, sessionID, strconv.Itoa(promptID), command.text, writeFrame, writeResponse, read)
				promptID++
				command.result <- err
				if err != nil {
					s.emit(SessionEvent{Kind: "error", SessionID: sessionID, Err: err})
					if errors.Is(err, errPromptTimeout) {
						return
					}
				} else {
					s.emit(SessionEvent{Kind: "turn_end", SessionID: sessionID})
				}
			}
		}
	}
}

func acpInitializeParams(opts Options) map[string]any {
	return map[string]any{
		"protocolVersion":    1,
		"cwd":                opts.Cwd,
		"provider":           opts.Provider,
		"model":              opts.Model,
		"clientCapabilities": map[string]any{},
	}
}

// writeFrameContext prevents a stalled carrier that stopped reading stdin
// from defeating the prompt deadline. The session is retired after the
// deadline, so a write that finishes later is contained to that dying child.
func writeFrameContext(ctx context.Context, writeFrame func(any) error, frame any) error {
	result := make(chan error, 1)
	go func() { result <- writeFrame(frame) }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Session) promptTurn(ctx context.Context, opts Options, sessionID, id, text string, writeFrame func(any) error, writeResponse func(rpcID, any) error, read func(context.Context) (rpcFrame, error)) error {
	promptCtx, cancel := context.WithTimeout(ctx, opts.PromptTimeout)
	defer cancel()
	if err := writeFrameContext(promptCtx, writeFrame, map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "session/prompt",
		"params": map[string]any{"sessionId": sessionID, "prompt": []map[string]string{{"type": "text", "text": text}}},
	}); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("ACP prompt timed out after %s: %w", opts.PromptTimeout, errPromptTimeout)
		}
		return err
	}
	for {
		frame, err := read(promptCtx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				// ACP defines session/cancel as a notification. It is best effort:
				// the session is retired below even when an older carrier ignores it.
				cancelCtx, cancelWrite := context.WithTimeout(context.Background(), 100*time.Millisecond)
				_ = writeFrameContext(cancelCtx, writeFrame, map[string]any{
					"jsonrpc": "2.0", "method": "session/cancel",
					"params": map[string]any{"sessionId": sessionID},
				})
				cancelWrite()
				return fmt.Errorf("ACP prompt timed out after %s: %w", opts.PromptTimeout, errPromptTimeout)
			}
			return err
		}
		if frame.Method != "" {
			if err := s.handleFrame(promptCtx, opts, frame, sessionID, writeResponse); err != nil {
				return err
			}
			continue
		}
		if string(frame.ID) != id {
			return fmt.Errorf("unexpected ACP prompt response id %q", frame.ID)
		}
		if frame.Error != nil {
			return fmt.Errorf("ACP session/prompt failed (%d): %s", frame.Error.Code, frame.Error.Message)
		}
		var result acpPromptResult
		if err := json.Unmarshal(frame.Result, &result); err != nil {
			return fmt.Errorf("decode ACP prompt response: %w", err)
		}
		if result.StopReason != "" {
			s.emit(SessionEvent{Kind: "status", SessionID: sessionID, Text: result.StopReason})
		}
		return nil
	}
}

func (s *Session) handleFrame(ctx context.Context, opts Options, frame rpcFrame, sessionID string, respond func(rpcID, any) error) error {
	switch frame.Method {
	case "session/update":
		var params acpSessionUpdate
		if err := json.Unmarshal(frame.Params, &params); err != nil {
			return fmt.Errorf("decode ACP session update: %w", err)
		}
		if sessionID != "" && params.SessionID != sessionID {
			return fmt.Errorf("ACP update targeted unexpected session %q", params.SessionID)
		}
		switch params.Update.SessionUpdate {
		case "agent_message_chunk":
			if text := acpTextContent(params.Update.Content); text != "" {
				s.emit(SessionEvent{Kind: "assistant", SessionID: params.SessionID, Text: text})
			}
		case "tool_call":
			event := SessionEvent{
				Kind: "tool_start", SessionID: params.SessionID,
				ToolID: params.Update.ToolCallID, ToolName: terminalSafe(params.Update.Title),
				ToolKind: terminalSafe(params.Update.Kind), ToolStatus: terminalSafe(params.Update.Status),
				ToolInput: safeACPToolInput(params.Update.RawInput),
			}
			for _, location := range params.Update.Locations {
				if safe := terminalSafe(location.Path); safe != "" {
					event.Locations = append(event.Locations, safe)
				}
			}
			s.emit(event)
		case "tool_call_update":
			event := SessionEvent{
				Kind: "tool_end", SessionID: params.SessionID,
				ToolID: params.Update.ToolCallID, ToolStatus: terminalSafe(params.Update.Status),
				ToolOutput: acpTextContent(params.Update.Content),
			}
			for _, location := range params.Update.Locations {
				if safe := terminalSafe(location.Path); safe != "" {
					event.Locations = append(event.Locations, safe)
				}
			}
			s.emit(event)
		case "plan":
			event := SessionEvent{Kind: "plan", SessionID: params.SessionID}
			for _, entry := range params.Update.Entries {
				content := terminalSafe(entry.Content)
				if content == "" {
					continue
				}
				event.Plan = append(event.Plan, workspacePlanItem{Content: content, Status: terminalSafe(entry.Status)})
			}
			s.emit(event)
		}
	case "session/request_permission":
		var params acpPermissionRequest
		if err := json.Unmarshal(frame.Params, &params); err != nil {
			return fmt.Errorf("decode ACP permission request: %w", err)
		}
		if sessionID != "" && params.SessionID != sessionID {
			return fmt.Errorf("ACP permission request targeted unexpected session %q", params.SessionID)
		}
		permissionID := string(frame.ID)
		s.emit(SessionEvent{Kind: "permission", SessionID: params.SessionID, PermissionID: permissionID, Text: "The agent wants to use a workspace tool."})
		select {
		case response := <-s.permissions:
			if response.id != permissionID {
				return fmt.Errorf("ACP permission response targeted unexpected request %q", response.id)
			}
			optionID := "reject-once"
			if response.allow {
				optionID = "allow-once"
			}
			return respond(frame.ID, map[string]any{"outcome": map[string]string{"outcome": "selected", "optionId": optionID}})
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			return errors.New("DeepSeek Harness session closed while waiting for permission")
		}
	}
	return nil
}

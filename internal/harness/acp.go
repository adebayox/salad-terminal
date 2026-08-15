package harness

// ACP is DeepSeek Harness's interactive process boundary. It is deliberately
// kept separate from dsh.go: ACP owns cancellation and one-shot approvals,
// while the lower-level SDK JSON-RPC client remains useful for headless tests.

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

const defaultACPCommand = "dsh-acp-demo"

type acpSessionUpdate struct {
	SessionID string `json:"sessionId"`
	Update    struct {
		SessionUpdate string `json:"sessionUpdate"`
		Content       struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"update"`
}

type acpPermissionRequest struct {
	SessionID string `json:"sessionId"`
}

type acpPromptResult struct {
	StopReason string `json:"stopReason"`
}

type acpInitializeResult struct {
	AgentCapabilities struct {
		LoadSession         bool `json:"loadSession"`
		SessionCapabilities struct {
			Resume json.RawMessage `json:"resume"`
		} `json:"sessionCapabilities"`
	} `json:"agentCapabilities"`
}

// RunACP starts one local ACP child, drives one prompt, handles the child's
// permission requests, and cancels the addressed session if ctx is cancelled.
func RunACP(ctx context.Context, opts Options, prompt string) (Result, error) {
	if strings.TrimSpace(prompt) == "" {
		return Result{}, errors.New("harness prompt cannot be empty")
	}
	if strings.TrimSpace(opts.Cwd) == "" {
		return Result{}, errors.New("harness workspace is required")
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
	if opts.SessionID == "" {
		id, err := newID()
		if err != nil {
			return Result{}, err
		}
		opts.SessionID = "salad-" + id
	}
	if opts.Output == nil {
		opts.Output = io.Discard
	}
	if opts.Input == nil {
		opts.Input = os.Stdin
	}
	inputReader := bufio.NewReader(opts.Input)

	cmd := exec.Command(opts.Command, opts.Args...)
	prepareProcessGroup(cmd)
	cmd.Dir = opts.Cwd
	cmd.Env = scrubbedEnvironment(withHarnessSafetyDefaults(opts.Env, opts.Cwd))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Result{}, fmt.Errorf("start harness stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("start harness stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, fmt.Errorf("start harness stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("start DeepSeek Harness ACP (%s): %w", opts.Command, err)
	}

	finished := make(chan struct{})
	var killOnce sync.Once
	var writeMu sync.Mutex
	var sessionMu sync.RWMutex
	currentSession := ""
	setSession := func(id string) {
		sessionMu.Lock()
		currentSession = id
		sessionMu.Unlock()
	}
	getSession := func() string {
		sessionMu.RLock()
		defer sessionMu.RUnlock()
		return currentSession
	}

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
		frame = append(frame, '}')
		frame = append(frame, '\n')
		writeMu.Lock()
		defer writeMu.Unlock()
		if _, err := stdin.Write(frame); err != nil {
			return fmt.Errorf("write ACP response: %w", err)
		}
		return nil
	}

	// ACP has a protocol-level cancellation notification. EOF remains the
	// cleanup fallback because the ACP server owns all sessions on this child.
	go func() {
		select {
		case <-ctx.Done():
			if sessionID := getSession(); sessionID != "" {
				_ = writeFrame(map[string]any{
					"jsonrpc": "2.0", "method": "session/cancel",
					"params": map[string]string{"sessionId": sessionID},
				})
			}
			_ = stdin.Close()
			select {
			case <-finished:
			case <-time.After(3 * time.Second):
				killOnce.Do(func() { _ = killProcessTree(cmd) })
			}
		case <-finished:
		}
	}()
	defer close(finished)
	defer func() { _ = stdin.Close(); _ = cmd.Wait() }()
	go func() { _, _ = io.Copy(io.Discard, stderr) }()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maxFrameBytes)
	read := func() (rpcFrame, error) {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return rpcFrame{}, fmt.Errorf("read ACP response: %w", err)
			}
			if ctx.Err() != nil {
				return rpcFrame{}, ctx.Err()
			}
			return rpcFrame{}, errors.New("DeepSeek Harness ACP closed its output")
		}
		return decodeFrame(scanner.Bytes())
	}
	request := func(id, method string, params any) (json.RawMessage, error) {
		if err := writeFrame(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
			return nil, err
		}
		for {
			frame, err := read()
			if err != nil {
				return nil, err
			}
			if frame.Method != "" {
				if err := handleACPFrame(opts, inputReader, writeResponse, frame, getSession()); err != nil {
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

	initializeResult, err := request("1", "initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{},
	})
	if err != nil {
		return Result{}, err
	}
	var initialized acpInitializeResult
	if err := json.Unmarshal(initializeResult, &initialized); err != nil {
		return Result{}, fmt.Errorf("decode ACP initialize response: %w", err)
	}

	sessionID := ""
	canResume := len(initialized.AgentCapabilities.SessionCapabilities.Resume) > 0 && string(initialized.AgentCapabilities.SessionCapabilities.Resume) != "null"
	if opts.SessionID != "" && (initialized.AgentCapabilities.LoadSession || canResume) {
		setSession(opts.SessionID)
		method := "session/resume"
		if initialized.AgentCapabilities.LoadSession {
			method = "session/load"
		}
		if _, err := request("2", method, map[string]any{
			"sessionId": opts.SessionID, "cwd": opts.Cwd, "mcpServers": []any{},
		}); err != nil {
			return Result{}, err
		}
		sessionID = opts.SessionID
	} else {
		if opts.SessionID != "" {
			fmt.Fprintln(opts.Output, "[harness] carrier does not advertise session restore; starting a fresh continuation")
		}
		newSessionParams, err := request("2", "session/new", map[string]any{
			"cwd": opts.Cwd, "mcpServers": []any{},
		})
		if err != nil {
			return Result{}, err
		}
		var session struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(newSessionParams, &session); err != nil || session.SessionID == "" {
			return Result{}, errors.New("ACP session/new returned no session id")
		}
		sessionID = session.SessionID
	}
	setSession(sessionID)

	if err := writeFrame(map[string]any{
		"jsonrpc": "2.0", "id": "3", "method": "session/prompt",
		"params": map[string]any{
			"sessionId": sessionID,
			"prompt":    []map[string]string{{"type": "text", "text": prompt}},
		},
	}); err != nil {
		return Result{}, err
	}
	for {
		frame, err := read()
		if err != nil {
			return Result{}, err
		}
		if frame.Method != "" {
			if err := handleACPFrame(opts, inputReader, writeResponse, frame, sessionID); err != nil {
				return Result{}, err
			}
			continue
		}
		if string(frame.ID) != "3" {
			return Result{}, fmt.Errorf("unexpected ACP prompt response id %q", frame.ID)
		}
		if frame.Error != nil {
			return Result{}, fmt.Errorf("ACP session/prompt failed (%d): %s", frame.Error.Code, frame.Error.Message)
		}
		var result acpPromptResult
		if err := json.Unmarshal(frame.Result, &result); err != nil {
			return Result{}, fmt.Errorf("decode ACP prompt response: %w", err)
		}
		if result.StopReason != "" {
			fmt.Fprintf(opts.Output, "[harness] %s\n", result.StopReason)
		}
		return Result{SessionID: sessionID}, nil
	}
}

func handleACPFrame(opts Options, input *bufio.Reader, respond func(rpcID, any) error, frame rpcFrame, sessionID string) error {
	switch frame.Method {
	case "session/update":
		var params acpSessionUpdate
		if err := json.Unmarshal(frame.Params, &params); err != nil {
			return fmt.Errorf("decode ACP session update: %w", err)
		}
		if params.SessionID == sessionID && params.Update.SessionUpdate == "agent_message_chunk" && params.Update.Content.Type == "text" {
			if text := terminalSafe(params.Update.Content.Text); text != "" {
				fmt.Fprint(opts.Output, text)
			}
		}
		return nil
	case "session/request_permission":
		var params acpPermissionRequest
		if err := json.Unmarshal(frame.Params, &params); err != nil {
			return fmt.Errorf("decode ACP permission request: %w", err)
		}
		if params.SessionID != sessionID {
			return fmt.Errorf("ACP permission request targeted unexpected session %q", params.SessionID)
		}
		fmt.Fprintln(opts.Output, "\n[harness] This action needs approval.")
		fmt.Fprint(opts.Output, "Allow once? [y/N] ")
		line, err := input.ReadString('\n')
		allow := err == nil && (strings.EqualFold(strings.TrimSpace(line), "y") || strings.EqualFold(strings.TrimSpace(line), "yes"))
		optionID := "reject-once"
		if allow {
			optionID = "allow-once"
		}
		return respond(frame.ID, map[string]any{"outcome": map[string]string{
			"outcome": "selected", "optionId": optionID,
		}})
	default:
		return nil
	}
}

func rpcIDJSON(id rpcID) []byte {
	text := string(id)
	if _, err := strconv.ParseInt(text, 10, 64); err == nil {
		return []byte(text)
	}
	encoded, _ := json.Marshal(text)
	return encoded
}

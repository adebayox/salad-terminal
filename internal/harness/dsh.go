package harness

// Package harness is the small process boundary for the optional DeepSeek
// Harness preview. It uses DSH's public stdio JSON-RPC protocol and does not
// import or duplicate the DSH runtime.

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	defaultCommand  = "dsh-jsonrpc-agent"
	defaultProvider = "deepseek-official"
	defaultModel    = "deepseek-v4-flash"
	maxFrameBytes   = 4 * 1024 * 1024
)

type Options struct {
	Command, Cwd, Provider, Model, SessionID string
	Args, Env                                []string
	Input                                    io.Reader
	InputCloser                              io.Closer
	Output                                   io.Writer
	// OnSessionID is called as soon as the runtime has identified the session.
	// The CLI uses it to make resume state durable before a long model turn or
	// an unexpected process exit can lose the in-memory result.
	OnSessionID func(string)
}

type Result struct{ SessionID string }

type rpcFrame struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      rpcID           `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcID accepts both JSON-RPC string and numeric ids. DSH's two public
// transports use the same wire convention but its implementations emit
// different id representations.
type rpcID string

func (id *rpcID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*id = ""
		return nil
	}
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		*id = rpcID(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*id = rpcID(number.String())
	return nil
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type sessionEventParams struct {
	SessionID string `json:"sessionId"`
	Event     struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	} `json:"event"`
}
type sessionStatusParams struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
}

// Run starts one local DSH child, sends one prompt, renders safe activity,
// and shuts that child down. Cancellation affects only this child.
func Run(ctx context.Context, opts Options, prompt string) (Result, error) {
	if strings.TrimSpace(prompt) == "" {
		return Result{}, errors.New("harness prompt cannot be empty")
	}
	if strings.TrimSpace(opts.Cwd) == "" {
		return Result{}, errors.New("harness workspace is required")
	}
	if opts.Command == "" {
		opts.Command = firstNonEmpty(os.Getenv("SALAD_DSH_COMMAND"), defaultCommand)
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

	cmd := exec.Command(opts.Command, opts.Args...)
	prepareProcessGroup(cmd)
	cmd.Dir, cmd.Env = opts.Cwd, scrubbedEnvironment(withHarnessSafetyDefaults(opts.Env, opts.Cwd))
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
		return Result{}, fmt.Errorf("start DeepSeek Harness (%s): %w", opts.Command, err)
	}
	if opts.OnSessionID != nil {
		opts.OnSessionID(opts.SessionID)
	}

	finished := make(chan struct{})
	var killOnce sync.Once
	go func() {
		select {
		case <-ctx.Done():
			// DSH treats stdin EOF as its graceful shutdown signal and uses
			// that path to clean up its own terminals/subprocesses. Escalate
			// only if it does not quiesce promptly.
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
				return rpcFrame{}, fmt.Errorf("read harness response: %w", err)
			}
			if ctx.Err() != nil {
				return rpcFrame{}, ctx.Err()
			}
			return rpcFrame{}, errors.New("DeepSeek Harness closed its output")
		}
		return decodeFrame(scanner.Bytes())
	}
	write := func(id, method string, params any) error {
		payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if err != nil {
			return fmt.Errorf("encode harness request: %w", err)
		}
		if _, err := fmt.Fprintf(stdin, "%s\n", payload); err != nil {
			return fmt.Errorf("write harness request: %w", err)
		}
		return nil
	}
	request := func(id, method string, params any) error {
		if err := write(id, method, params); err != nil {
			return err
		}
		for {
			frame, err := read()
			if err != nil {
				return err
			}
			if frame.Method != "" {
				renderNotification(opts.Output, frame, opts.SessionID)
				continue
			}
			if string(frame.ID) != id {
				return fmt.Errorf("unexpected harness response id %q", frame.ID)
			}
			if frame.Error != nil {
				return fmt.Errorf("harness %s failed (%d): %s", method, frame.Error.Code, frame.Error.Message)
			}
			return nil
		}
	}

	if err := request("1", "initialize", map[string]any{"cwd": opts.Cwd, "provider": opts.Provider, "model": opts.Model}); err != nil {
		return Result{}, err
	}
	if err := write("2", "session/prompt", map[string]any{"sessionId": opts.SessionID, "contentBlocks": []map[string]string{{"type": "text", "text": prompt}}}); err != nil {
		return Result{}, err
	}
	for {
		frame, err := read()
		if err != nil {
			return Result{}, err
		}
		if frame.Method == "session.event" {
			if renderSessionEvent(opts.Output, frame, opts.SessionID) {
				break
			}
			continue
		}
		if frame.Method == "session.status" {
			var status sessionStatusParams
			if err := json.Unmarshal(frame.Params, &status); err != nil {
				return Result{}, fmt.Errorf("decode harness status: %w", err)
			}
			renderNotification(opts.Output, frame, opts.SessionID)
			if status.SessionID == opts.SessionID && status.Status == "idle" {
				break
			}
			continue
		}
		if frame.Method != "" {
			renderNotification(opts.Output, frame, opts.SessionID)
			continue
		}
		if frame.ID == "2" {
			continue
		}
		return Result{}, fmt.Errorf("unexpected harness frame while running: %s", frame.ID)
	}
	_ = request("3", "shutdown", map[string]any{})
	return Result{SessionID: opts.SessionID}, nil
}

func renderNotification(out io.Writer, frame rpcFrame, sessionID string) {
	if frame.Method != "session.status" {
		return
	}
	var status sessionStatusParams
	if json.Unmarshal(frame.Params, &status) == nil && status.SessionID == sessionID {
		fmt.Fprintf(out, "[harness] %s\n", status.Status)
	}
}

func renderSessionEvent(out io.Writer, frame rpcFrame, sessionID string) bool {
	var params sessionEventParams
	if json.Unmarshal(frame.Params, &params) != nil || params.SessionID != sessionID {
		return false
	}
	switch params.Event.Type {
	case "assistant/message":
		if text := contentText(params.Event.Data); text != "" {
			fmt.Fprintln(out, terminalSafe(text))
		}
	case "tool/call":
		fmt.Fprintln(out, "[harness] tool requested")
	case "approval/asked":
		fmt.Fprintln(out, "[harness] approval requested")
	case "turn/end":
		return true
	}
	return false
}

func contentText(data json.RawMessage) string {
	var envelope map[string]any
	if json.Unmarshal(data, &envelope) != nil {
		return ""
	}
	message, _ := envelope["message"].(map[string]any)
	content, ok := message["content"].([]any)
	if !ok {
		content, ok = envelope["content"].([]any)
	}
	if !ok {
		return ""
	}
	var parts []string
	for _, item := range content {
		block, ok := item.(map[string]any)
		if ok && block["type"] == "text" {
			if text, ok := block["text"].(string); ok {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "")
}

func decodeFrame(line []byte) (rpcFrame, error) {
	if len(line) == 0 {
		return rpcFrame{}, errors.New("empty harness frame")
	}
	if len(line) > maxFrameBytes {
		return rpcFrame{}, fmt.Errorf("harness frame exceeds %d bytes", maxFrameBytes)
	}
	var frame rpcFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return rpcFrame{}, fmt.Errorf("decode harness frame: %w", err)
	}
	if frame.JSONRPC != "" && frame.JSONRPC != "2.0" {
		return rpcFrame{}, fmt.Errorf("unsupported harness protocol %q", frame.JSONRPC)
	}
	if frame.ID == "" && frame.Method == "" {
		return rpcFrame{}, errors.New("harness frame has no id or method")
	}
	return frame, nil
}

func newID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// scrubbedEnvironment never forwards Salad credentials or arbitrary secrets.
func scrubbedEnvironment(overlay []string) []string {
	allowed := map[string]bool{"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "TMPDIR": true, "TMP": true, "TEMP": true, "TERM": true, "COLORTERM": true, "LANG": true, "LC_ALL": true, "SHELL": true, "SYSTEMROOT": true, "ComSpec": true, "XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "XDG_CACHE_HOME": true}
	values := map[string]string{}
	for _, entry := range append(os.Environ(), overlay...) {
		name, value, ok := strings.Cut(entry, "=")
		if ok && safeEnvironmentName(name, allowed) {
			values[name] = value
		}
	}
	// Network access is a capability, not ordinary child configuration. A
	// parent shell's DSH_NETWORK_MODE must never silently survive into a run;
	// only the explicit per-run overlay may select allow.
	networkMode := "deny"
	for _, entry := range overlay {
		if name, value, ok := strings.Cut(entry, "="); ok && name == "DSH_NETWORK_MODE" {
			networkMode = value
		}
	}
	values["DSH_NETWORK_MODE"] = networkMode
	out := make([]string, 0, len(values))
	for name, value := range values {
		out = append(out, name+"="+value)
	}
	return out
}

// withHarnessSafetyDefaults keeps model-controlled shell commands inside the
// terminal's safer default. Network mode is supplied only by the caller's
// explicit per-run overlay; a parent-shell value cannot silently opt in.
func withHarnessSafetyDefaults(overlay []string, cwd string) []string {
	values := append([]string{}, overlay...)
	has := func(name string) bool {
		for _, entry := range values {
			if key, _, ok := strings.Cut(entry, "="); ok && key == name {
				return true
			}
		}
		return false
	}
	if !has("DSH_NETWORK_MODE") {
		values = append(values, "DSH_NETWORK_MODE=deny")
	}
	if !has("DSH_CWD") && strings.TrimSpace(cwd) != "" {
		values = append(values, "DSH_CWD="+cwd)
	}
	return values
}

func safeEnvironmentName(name string, allowed map[string]bool) bool {
	return allowed[name] || strings.HasPrefix(name, "DSH_") || strings.HasPrefix(name, "DEEPSEEK_") || strings.HasPrefix(name, "SALAD_DSH_")
}

// terminalSafe prevents model output from changing the terminal title,
// cursor, colours, or command line. Newlines and tabs remain readable.
func terminalSafe(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] == 0x1b {
			i++
			if i < len(value) && value[i] == ']' { // OSC, including title changes.
				i++
				for i < len(value) {
					if value[i] == 0x07 {
						i++
						break
					}
					if value[i] == 0x1b && i+1 < len(value) && value[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
				continue
			}
			if i < len(value) && value[i] == '[' { // CSI, including ANSI colours.
				i++
				for i < len(value) {
					ch := value[i]
					i++
					if ch >= 0x40 && ch <= 0x7e {
						break
					}
				}
				continue
			}
			continue
		}
		if (value[i] < 0x20 && value[i] != '\n' && value[i] != '\t') || value[i] == 0x7f {
			i++
			continue
		}
		out.WriteByte(value[i])
		i++
	}
	return out.String()
}

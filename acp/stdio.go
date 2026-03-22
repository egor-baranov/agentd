package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"agentd/control"
)

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type StdioClient struct {
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	closers []io.Closer

	enc *json.Encoder
	dec *json.Decoder

	mu      sync.Mutex
	pending map[string]chan rpcEnvelope
	notify  func(StreamEvent)
	closed  chan struct{}
}

func StartCommand(ctx context.Context, command string, args []string, dir string, env []string, stderr io.Writer) (*StdioClient, *exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	client := NewClient(stdout, stdin)
	return client, cmd, nil
}

func NewClient(stdout io.ReadCloser, stdin io.WriteCloser) *StdioClient {
	c := &StdioClient{
		stdin:   stdin,
		stdout:  stdout,
		closers: []io.Closer{stdin, stdout},
		enc:     json.NewEncoder(stdin),
		dec:     json.NewDecoder(stdout),
		pending: map[string]chan rpcEnvelope{},
		closed:  make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func (c *StdioClient) SetNotificationHandler(fn func(StreamEvent)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notify = fn
}

func (c *StdioClient) Initialize(ctx context.Context) (control.ACPNegotiation, error) {
	var resp InitializeResponse
	if err := c.call(ctx, "initialize", InitializeRequest{ClientName: "agentd", ClientVersion: "0.1.0"}, &resp); err != nil {
		return control.ACPNegotiation{}, err
	}
	return control.ACPNegotiation{ProtocolVersion: resp.ProtocolVersion, Capabilities: resp.Capabilities, AuthModes: resp.AuthMethods}, nil
}

func (c *StdioClient) NewSession(ctx context.Context, req control.SessionSpec) (control.ACPNegotiation, error) {
	var resp SessionResponse
	if err := c.call(ctx, "session/new", SessionNewParams{SessionID: req.ResumeSessionID, Metadata: req.Metadata, MCPServers: req.MCPServers}, &resp); err != nil {
		return control.ACPNegotiation{}, err
	}
	return control.ACPNegotiation{SessionMetadata: resp.Metadata}, nil
}

func (c *StdioClient) LoadSession(ctx context.Context, sessionID string) (control.ACPNegotiation, error) {
	var resp SessionResponse
	if err := c.call(ctx, "session/load", SessionLoadParams{SessionID: sessionID}, &resp); err != nil {
		return control.ACPNegotiation{}, err
	}
	return control.ACPNegotiation{SessionMetadata: resp.Metadata}, nil
}

func (c *StdioClient) Prompt(ctx context.Context, sessionID, runID string, input control.SessionInput) error {
	return c.call(ctx, "session/prompt", PromptParams{SessionID: sessionID, RunID: runID, Prompt: input.Prompt, Metadata: input.Metadata}, nil)
}

func (c *StdioClient) Cancel(ctx context.Context, sessionID, runID string) error {
	return c.call(ctx, "session/cancel", CancelParams{SessionID: sessionID, RunID: runID}, nil)
}

func (c *StdioClient) ListSessions(ctx context.Context) ([]string, error) {
	var resp SessionListResponse
	if err := c.call(ctx, "session/list", map[string]any{}, &resp); err != nil {
		return nil, err
	}
	return resp.SessionIDs, nil
}

func (c *StdioClient) Close() error {
	var first error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (c *StdioClient) call(ctx context.Context, method string, params any, out any) error {
	id := control.NewID("rpc")
	wait := make(chan rpcEnvelope, 1)

	c.mu.Lock()
	c.pending[id] = wait
	err := c.enc.Encode(rpcEnvelope{JSONRPC: "2.0", ID: id, Method: method, Params: mustJSON(params)})
	c.mu.Unlock()
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case env, ok := <-wait:
		if !ok {
			return io.EOF
		}
		if env.Error != nil {
			return fmt.Errorf("acp %s: %s", method, env.Error.Message)
		}
		if out != nil && len(env.Result) > 0 {
			if err := json.Unmarshal(env.Result, out); err != nil {
				return err
			}
		}
		return nil
	}
}

func (c *StdioClient) readLoop() {
	defer close(c.closed)
	for {
		var env rpcEnvelope
		if err := c.dec.Decode(&env); err != nil {
			c.failAll(err)
			return
		}
		if env.ID != "" {
			c.mu.Lock()
			ch := c.pending[env.ID]
			delete(c.pending, env.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- env
				close(ch)
			}
			continue
		}
		if env.Method == "session/update" {
			var event StreamEvent
			if err := json.Unmarshal(env.Params, &event); err == nil {
				c.mu.Lock()
				notify := c.notify
				c.mu.Unlock()
				if notify != nil {
					notify(event)
				}
			}
		}
	}
}

func (c *StdioClient) failAll(err error) {
	if errors.Is(err, io.EOF) {
		err = fmt.Errorf("acp subprocess exited")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, ch := range c.pending {
		ch <- rpcEnvelope{ID: id, Error: &rpcError{Code: -32000, Message: err.Error()}}
		close(ch)
		delete(c.pending, id)
	}
}

func mustJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	raw, _ := json.Marshal(v)
	return raw
}

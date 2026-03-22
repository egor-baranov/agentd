package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type envelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type sessionSpec struct {
	SessionID string         `json:"session_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type promptParams struct {
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id"`
	Prompt    string         `json:"prompt,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type cancelParams struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
}

type streamEvent struct {
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id,omitempty"`
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Final     bool           `json:"final,omitempty"`
	Error     string         `json:"error,omitempty"`
}

type state struct {
	mu       sync.Mutex
	sessions map[string]map[string]any
	runs     map[string]context.CancelFunc
	encMu    sync.Mutex
}

func main() {
	st := &state{sessions: map[string]map[string]any{}, runs: map[string]context.CancelFunc{}}
	dec := json.NewDecoder(bufio.NewReader(os.Stdin))
	enc := json.NewEncoder(os.Stdout)
	for {
		var env envelope
		if err := dec.Decode(&env); err != nil {
			return
		}
		switch env.Method {
		case "initialize":
			st.encode(enc, envelope{JSONRPC: "2.0", ID: env.ID, Result: map[string]any{"protocol_version": "2025-10", "capabilities": []string{"session/new", "session/load", "session/prompt", "session/cancel", "session/list"}, "auth_methods": []string{"agent"}}})
		case "session/new":
			var req sessionSpec
			_ = json.Unmarshal(env.Params, &req)
			if req.SessionID == "" {
				req.SessionID = fmt.Sprintf("sess-%d", time.Now().UnixNano())
			}
			st.mu.Lock()
			st.sessions[req.SessionID] = req.Metadata
			st.mu.Unlock()
			st.encode(enc, envelope{JSONRPC: "2.0", ID: env.ID, Result: map[string]any{"session_id": req.SessionID, "metadata": req.Metadata}})
		case "session/load":
			var req sessionSpec
			_ = json.Unmarshal(env.Params, &req)
			st.mu.Lock()
			meta, ok := st.sessions[req.SessionID]
			st.mu.Unlock()
			if !ok {
				st.encode(enc, envelope{JSONRPC: "2.0", ID: env.ID, Error: &rpcError{Code: -32004, Message: "unknown session"}})
				continue
			}
			st.encode(enc, envelope{JSONRPC: "2.0", ID: env.ID, Result: map[string]any{"session_id": req.SessionID, "metadata": meta}})
		case "session/list":
			st.mu.Lock()
			ids := make([]string, 0, len(st.sessions))
			for id := range st.sessions {
				ids = append(ids, id)
			}
			st.mu.Unlock()
			st.encode(enc, envelope{JSONRPC: "2.0", ID: env.ID, Result: map[string]any{"session_ids": ids}})
		case "session/prompt":
			var req promptParams
			_ = json.Unmarshal(env.Params, &req)
			ctx, cancel := context.WithCancel(context.Background())
			st.mu.Lock()
			st.runs[req.RunID] = cancel
			st.mu.Unlock()
			st.encode(enc, envelope{JSONRPC: "2.0", ID: env.ID, Result: map[string]any{"accepted": true}})
			go func() {
				defer func() {
					st.mu.Lock()
					delete(st.runs, req.RunID)
					st.mu.Unlock()
				}()
				parts := []string{"echo:", req.Prompt}
				for _, part := range parts {
					select {
					case <-ctx.Done():
						st.encode(enc, envelope{JSONRPC: "2.0", Method: "session/update", Params: mustJSON(streamEvent{SessionID: req.SessionID, RunID: req.RunID, Type: "cancelled", Final: true})})
						return
					case <-time.After(40 * time.Millisecond):
						st.encode(enc, envelope{JSONRPC: "2.0", Method: "session/update", Params: mustJSON(streamEvent{SessionID: req.SessionID, RunID: req.RunID, Type: "chunk", Text: part})})
					}
				}
				st.encode(enc, envelope{JSONRPC: "2.0", Method: "session/update", Params: mustJSON(streamEvent{SessionID: req.SessionID, RunID: req.RunID, Type: "completed", Text: req.Prompt, Final: true, Metadata: map[string]any{"echo": req.Prompt}})})
			}()
		case "session/cancel":
			var req cancelParams
			_ = json.Unmarshal(env.Params, &req)
			st.mu.Lock()
			cancel := st.runs[req.RunID]
			st.mu.Unlock()
			if cancel != nil {
				cancel()
			}
			st.encode(enc, envelope{JSONRPC: "2.0", ID: env.ID, Result: map[string]any{"cancelled": cancel != nil}})
		default:
			st.encode(enc, envelope{JSONRPC: "2.0", ID: env.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
		}
	}
}

func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

func (s *state) encode(enc *json.Encoder, env envelope) {
	s.encMu.Lock()
	defer s.encMu.Unlock()
	_ = enc.Encode(env)
}

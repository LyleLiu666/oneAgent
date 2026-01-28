package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type Client struct {
	conn *jsonrpcConn

	mu        sync.Mutex
	nextID    int
	pending   map[int]chan rpcResponse
	readOnce  sync.Once
	readErr   error
	readClose chan struct{}
}

func newClient(r ioReader, w ioWriter) *Client {
	return &Client{
		conn:      newJSONRPCConn(r, w),
		nextID:    1,
		pending:   make(map[int]chan rpcResponse, 64),
		readClose: make(chan struct{}),
	}
}

// ioReader/ioWriter are small aliases to keep this file self-contained.
type ioReader interface{ Read([]byte) (int, error) }
type ioWriter interface{ Write([]byte) (int, error) }

func (c *Client) startReadLoop() {
	c.readOnce.Do(func() {
		go func() {
			defer close(c.readClose)
			for {
				msg, err := c.conn.readMessage()
				if err != nil {
					c.mu.Lock()
					c.readErr = err
					// Fail all pending.
					for id, ch := range c.pending {
						delete(c.pending, id)
						close(ch)
					}
					c.mu.Unlock()
					return
				}

				var resp rpcResponse
				if err := json.Unmarshal(msg, &resp); err != nil {
					continue
				}
				id, ok := resp.ID.(float64)
				if !ok {
					// Notifications and non-standard messages are ignored.
					continue
				}
				reqID := int(id)

				c.mu.Lock()
				ch := c.pending[reqID]
				delete(c.pending, reqID)
				c.mu.Unlock()
				if ch == nil {
					continue
				}
				ch <- resp
				close(ch)
			}
		}()
	})
}

func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	if c == nil || c.conn == nil {
		return errors.New("lsp client is nil")
	}
	method = strings.TrimSpace(method)
	if method == "" {
		return errors.New("method is required")
	}

	c.startReadLoop()

	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan rpcResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	raw, _ := json.Marshal(req)
	if err := c.conn.writeMessage(raw); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			c.mu.Lock()
			err := c.readErr
			c.mu.Unlock()
			if err != nil {
				return err
			}
			return errors.New("lsp server closed")
		}
		if resp.Error != nil {
			return fmt.Errorf("lsp error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if out == nil {
			return nil
		}
		if len(resp.Result) == 0 {
			return errors.New("empty result")
		}
		return json.Unmarshal(resp.Result, out)
	}
}

func (c *Client) Notify(method string, params any) error {
	if c == nil || c.conn == nil {
		return errors.New("lsp client is nil")
	}
	method = strings.TrimSpace(method)
	if method == "" {
		return errors.New("method is required")
	}
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	raw, _ := json.Marshal(req)
	return c.conn.writeMessage(raw)
}

type Session struct {
	Cmd    *exec.Cmd
	Client *Client
}

func StartSession(ctx context.Context, command string, args []string, cwd string, env []string) (*Session, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("command is required")
	}
	cmd := exec.CommandContext(ctx, command, args...)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	// Merge stderr to stdout for now; servers often log there. This avoids deadlocks.
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}

	c := newClient(stdout, stdin)
	return &Session{Cmd: cmd, Client: c}, nil
}

func (s *Session) Close(ctx context.Context) error {
	if s == nil || s.Cmd == nil {
		return nil
	}
	// Best-effort: ask server to shutdown, then exit.
	if s.Client != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_ = s.Client.Call(shutdownCtx, "shutdown", nil, nil)
		cancel()
		_ = s.Client.Notify("exit", nil)
	}
	done := make(chan error, 1)
	go func() { done <- s.Cmd.Wait() }()
	select {
	case <-ctx.Done():
		_ = s.Cmd.Process.Kill()
		<-done
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func Initialize(ctx context.Context, c *Client, workspaceRoot string) error {
	if c == nil {
		return errors.New("client is nil")
	}
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return errors.New("workspaceRoot is required")
	}
	rootURI := PathToFileURI(workspaceRoot)
	params := map[string]any{
		"processId": nil,
		"rootUri":   rootURI,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"definition": map[string]any{},
				"references": map[string]any{},
				"rename":     map[string]any{},
			},
		},
		"workspaceFolders": []map[string]any{
			{"uri": rootURI, "name": filepath.Base(workspaceRoot)},
		},
	}
	var resp any
	initCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	err := c.Call(initCtx, "initialize", params, &resp)
	cancel()
	if err != nil {
		return err
	}
	_ = c.Notify("initialized", map[string]any{})
	return nil
}

func PathToFileURI(p string) string {
	p = filepath.Clean(p)
	u := &url.URL{Scheme: "file"}
	// url.URL requires path to be slash-separated.
	u.Path = filepath.ToSlash(p)
	return u.String()
}

func FileURIToPath(uri string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported uri scheme: %s", u.Scheme)
	}
	p := u.Path
	if p == "" {
		return "", errors.New("empty uri path")
	}
	p = filepath.FromSlash(p)
	return filepath.Clean(p), nil
}

package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// jsonrpcConn implements the minimal JSON-RPC 2.0 framing used by LSP (stdio):
// headers (Content-Length) + "\r\n\r\n" + JSON payload.
type jsonrpcConn struct {
	r  *bufio.Reader
	w  io.Writer
	mu sync.Mutex
}

func newJSONRPCConn(r io.Reader, w io.Writer) *jsonrpcConn {
	return &jsonrpcConn{
		r: bufio.NewReader(r),
		w: w,
	}
}

func (c *jsonrpcConn) writeMessage(payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	if _, err := c.w.Write(payload); err != nil {
		return err
	}
	return nil
}

func (c *jsonrpcConn) readMessage() ([]byte, error) {
	// Read headers.
	var contentLength int
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(strings.ToLower(parts[0]))
		v := strings.TrimSpace(parts[1])
		if k == "content-length" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid Content-Length: %q", v)
			}
			contentLength = n
		}
	}
	if contentLength <= 0 {
		return nil, errors.New("missing Content-Length")
	}

	buf := make([]byte, contentLength)
	if _, err := io.ReadFull(c.r, buf); err != nil {
		return nil, err
	}
	// Ensure it's valid JSON for better errors upstream.
	if !json.Valid(buf) {
		return nil, fmt.Errorf("invalid json payload: %s", string(bytes.TrimSpace(buf)))
	}
	return buf, nil
}


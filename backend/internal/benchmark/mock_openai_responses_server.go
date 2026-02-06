package benchmark

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
)

// MockOpenAIResponsesServer is a deterministic mock for the OpenAI Responses API.
//
// It responds to POST /responses with a scripted sequence of tool calls / text deltas.
// If the script is exhausted, it returns HTTP 500 to fail fast.
type MockOpenAIResponsesServer struct {
	srv *httptest.Server

	mu        sync.Mutex
	nextIndex int
	responses []MockResponse
	requests  [][]byte
}

func StartMockOpenAIResponsesServer(responses []MockResponse) *MockOpenAIResponsesServer {
	m := &MockOpenAIResponsesServer{
		responses: append([]MockResponse(nil), responses...),
	}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	return m
}

func (m *MockOpenAIResponsesServer) URL() string {
	if m == nil || m.srv == nil {
		return ""
	}
	return m.srv.URL
}

func (m *MockOpenAIResponsesServer) Close() {
	if m == nil || m.srv == nil {
		return
	}
	m.srv.Close()
}

func (m *MockOpenAIResponsesServer) Requests() [][]byte {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]byte, 0, len(m.requests))
	for _, b := range m.requests {
		cp := make([]byte, len(b))
		copy(cp, b)
		out = append(out, cp)
	}
	return out
}

type responsesEvent struct {
	Type  string         `json:"type"`
	Item  *responsesItem `json:"item,omitempty"`
	Delta string         `json:"delta,omitempty"`
}

type responsesItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (m *MockOpenAIResponsesServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/responses" {
		http.NotFound(w, r)
		return
	}
	body, _ := io.ReadAll(r.Body)

	m.mu.Lock()
	m.requests = append(m.requests, body)
	idx := m.nextIndex
	m.nextIndex++
	var resp MockResponse
	ok := idx >= 0 && idx < len(m.responses)
	if ok {
		resp = m.responses[idx]
	}
	m.mu.Unlock()

	if !ok {
		http.Error(w, fmt.Sprintf("mock script exhausted at index=%d", idx), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")

	for i, tc := range resp.ToolCalls {
		item := &responsesItem{
			Type:      "function_call",
			ID:        "fc_" + strconv.Itoa(i+1),
			CallID:    tc.CallID,
			Name:      tc.Name,
			Arguments: string(tc.Arguments),
		}
		ev := responsesEvent{Type: "response.output_item.added", Item: item}
		b, _ := json.Marshal(ev)
		_, _ = io.WriteString(w, "data: "+string(b)+"\n\n")
	}

	if resp.Text != "" {
		ev := responsesEvent{Type: "response.output_text.delta", Delta: resp.Text}
		b, _ := json.Marshal(ev)
		_, _ = io.WriteString(w, "data: "+string(b)+"\n\n")
	}

	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}


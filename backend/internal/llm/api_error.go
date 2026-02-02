package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// APIError is a structured error returned by LLM providers (best-effort parsed).
//
// It is intentionally provider-agnostic; individual clients can populate fields
// when their error payload format is known.
type APIError struct {
	StatusCode int
	Message    string
	Type       string
	RequestID  string
	Raw        string
}

func (e *APIError) Error() string {
	head := fmt.Sprintf("API error (status %d", e.StatusCode)
	if strings.TrimSpace(e.RequestID) != "" {
		head += ", request_id=" + strings.TrimSpace(e.RequestID)
	}
	if strings.TrimSpace(e.Type) != "" {
		head += ", type=" + strings.TrimSpace(e.Type)
	}
	head += ")"

	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = strings.TrimSpace(e.Raw)
	}
	if msg == "" {
		return head
	}
	return head + ": " + msg
}

type openAIErrorEnvelope struct {
	// Standard OpenAI error envelope: {"error":{"message":"...","type":"...","code":...}}
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code,omitempty"`
	} `json:"error,omitempty"`

	// Some OpenAI-compatible providers use a top-level "type":"error" and a nested error object.
	Type      string `json:"type,omitempty"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	// Alternate casing seen in the wild.
	RequestId string `json:"requestId,omitempty"`
}

func openAIAPIErrorFromResponse(resp *http.Response, body []byte) error {
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	raw := strings.TrimSpace(string(bytes.TrimSpace(body)))

	apiErr := &APIError{
		StatusCode: status,
		Raw:        raw,
	}
	if resp != nil {
		apiErr.RequestID = strings.TrimSpace(resp.Header.Get("x-request-id"))
	}

	var parsed openAIErrorEnvelope
	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.Error != nil {
			apiErr.Message = parsed.Error.Message
			apiErr.Type = parsed.Error.Type
		}
		if apiErr.Type == "" {
			apiErr.Type = parsed.Type
		}
		if apiErr.Message == "" {
			apiErr.Message = parsed.Message
		}
		if rid := strings.TrimSpace(parsed.RequestID); rid != "" {
			apiErr.RequestID = rid
		} else if rid := strings.TrimSpace(parsed.RequestId); rid != "" {
			apiErr.RequestID = rid
		}
	}

	return apiErr
}

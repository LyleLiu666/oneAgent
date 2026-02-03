package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SecretaryState_NoSessionID_BootstrapsCanonicalSession(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body := mustGet(t, srv.URL, rt.AuthToken, "/api/secretary/state")
	var st struct {
		SessionID       string `json:"session_id"`
		CursorMessageID uint   `json:"cursor_message_id"`
	}
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if strings.TrimSpace(st.SessionID) == "" {
		t.Fatalf("expected session_id, got %+v", st)
	}
	if st.CursorMessageID != 0 {
		t.Fatalf("expected cursor=0 for new session, got %+v", st)
	}

	// The canonical session should now exist and be readable via secretary API.
	body2 := mustGet(t, srv.URL, rt.AuthToken, "/api/secretary/session")
	var sess struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body2, &sess); err != nil {
		t.Fatalf("unmarshal secretary session: %v", err)
	}
	if sess.ID != st.SessionID {
		t.Fatalf("expected secretary session id=%q, got %+v", st.SessionID, sess)
	}
}

func TestServer_SecretaryInboxMessage_NoAck_AndCanonicalSessionStable(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	var r1 struct {
		SessionID string `json:"session_id"`
		AckText   string `json:"ack_text"`
		AckID     uint   `json:"ack_message_id"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"content": "你好",
	}, &r1)
	if strings.TrimSpace(r1.SessionID) == "" {
		t.Fatalf("expected session id")
	}
	if strings.TrimSpace(r1.AckText) != "" || r1.AckID != 0 {
		t.Fatalf("expected empty ack, got ack_message_id=%d ack_text=%q", r1.AckID, r1.AckText)
	}

	var r2 struct {
		SessionID string `json:"session_id"`
		AckText   string `json:"ack_text"`
		AckID     uint   `json:"ack_message_id"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"content": "再来一次",
	}, &r2)
	if r2.SessionID != r1.SessionID {
		t.Fatalf("expected canonical session id to be stable, got %q then %q", r1.SessionID, r2.SessionID)
	}
	if strings.TrimSpace(r2.AckText) != "" || r2.AckID != 0 {
		t.Fatalf("expected empty ack, got ack_message_id=%d ack_text=%q", r2.AckID, r2.AckText)
	}
}

package runtime

import (
	"context"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

func TestRuntime_Init_Health_AndStorageRoundTrip(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	rt1 := rt
	t.Cleanup(func() { _ = rt1.Close() })

	ctx := context.Background()
	health, err := rt.Health(ctx)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health.Status == "" {
		t.Fatalf("expected health status")
	}
	if !health.SettingsDBOK || !health.MemoryDBOK || !health.DataDirOK || !health.LogsDirOK {
		t.Fatalf("expected healthy storage, got %+v", health)
	}

	// Canonical secretary session id is stable.
	sec1, err := rt.ResolveSecretarySessionID(ctx, "local")
	if err != nil {
		t.Fatalf("resolve secretary session id: %v", err)
	}
	sec2, err := rt.ResolveSecretarySessionID(ctx, "local")
	if err != nil {
		t.Fatalf("resolve secretary session id again: %v", err)
	}
	if sec1 == "" || sec2 == "" || sec1 != sec2 {
		t.Fatalf("expected stable secretary session id, got %q and %q", sec1, sec2)
	}

	// Settings SQLite round-trip.
	userID := "local"
	_, err = rt.Settings.CreateProvider(ctx, settingsdb.Provider{
		ID:           "p1",
		UserID:       userID,
		Name:         "Test",
		ProviderType: "openai",
		BaseURL:      "https://example.invalid",
		APIKey:       "sk-test",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	providers, err := rt.Settings.ListProviders(ctx, userID)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].ID != "p1" || providers[0].UserID != userID {
		t.Fatalf("unexpected provider: %+v", providers[0])
	}

	// Session store round-trip.
	sessionID := "s1"
	_, err = rt.Sessions.GetOrCreateSession(sessionID, userID, "assistant", "hello")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := rt.Sessions.AppendMessage(sessionID, model.ChatMessage{
		Role:    model.MessageRoleUser,
		Type:    model.MessageTypeText,
		Content: "hi",
	}); err != nil {
		t.Fatalf("append message: %v", err)
	}
	_, msgs, err := rt.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Role != model.MessageRoleUser || msgs[0].Content != "hi" {
		t.Fatalf("unexpected message: %+v", msgs[0])
	}

	// Restart keeps secretary session id stable (best-effort).
	_ = rt1.Close()
	rt, err = Init(cfg)
	if err != nil {
		t.Fatalf("init runtime after restart: %v", err)
	}
	rt2 := rt
	t.Cleanup(func() { _ = rt2.Close() })

	sec3, err := rt2.ResolveSecretarySessionID(ctx, "local")
	if err != nil {
		t.Fatalf("resolve secretary session id after restart: %v", err)
	}
	if sec3 != sec1 {
		t.Fatalf("expected same secretary session id after restart, got %q want %q", sec3, sec1)
	}
}

package settingsdb

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAuthTokens_CreateLookupRevoke(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	tok, err := db.CreateAuthToken(ctx, "alice")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if tok.Token == "" || tok.PrincipalID != "alice" {
		t.Fatalf("unexpected token: %+v", tok)
	}

	got, ok, err := db.LookupAuthToken(ctx, tok.Token)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !ok || got != "alice" {
		t.Fatalf("expected alice, got ok=%v principal=%s", ok, got)
	}

	if err := db.RevokeAuthToken(ctx, tok.Token); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	got, ok, err = db.LookupAuthToken(ctx, tok.Token)
	if err != nil {
		t.Fatalf("lookup after revoke: %v", err)
	}
	if ok {
		t.Fatalf("expected revoked token to be invalid")
	}
}


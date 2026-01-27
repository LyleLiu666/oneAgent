package tool

import (
	"context"
	"strings"
	"sync"
)

type occState struct {
	mu           sync.Mutex
	fingerprints map[string]string // abs path -> sha256 hex
}

type contextKeyOCC string

const occContextKey contextKeyOCC = "occ"

func ContextWithOCC(ctx context.Context, enabled bool) context.Context {
	if !enabled {
		return ctx
	}
	if OCCFromContext(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, occContextKey, &occState{fingerprints: make(map[string]string)})
}

func OCCFromContext(ctx context.Context) *occState {
	if v, ok := ctx.Value(occContextKey).(*occState); ok {
		return v
	}
	return nil
}

func occExpectedSHA256(ctx context.Context, absPath string) (string, bool) {
	state := OCCFromContext(ctx)
	if state == nil {
		return "", false
	}
	absPath = strings.TrimSpace(absPath)
	if absPath == "" {
		return "", false
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	sha, ok := state.fingerprints[absPath]
	return sha, ok && sha != ""
}

func occRecordSHA256(ctx context.Context, absPath, sha string) {
	state := OCCFromContext(ctx)
	if state == nil {
		return
	}
	absPath = strings.TrimSpace(absPath)
	sha = strings.TrimSpace(sha)
	if absPath == "" || sha == "" {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.fingerprints[absPath] = sha
}


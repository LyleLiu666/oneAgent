package runtime

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureAuthToken loads token from the given path, generating and persisting it if missing.
// The token is an opaque string (not JWT) and never expires.
func EnsureAuthToken(tokenPath string) (string, bool, error) {
	if strings.TrimSpace(tokenPath) == "" {
		return "", false, errors.New("tokenPath is required")
	}

	data, err := os.ReadFile(tokenPath)
	if err == nil {
		token := strings.TrimSpace(string(data))
		if token == "" {
			return "", false, fmt.Errorf("auth token file is empty: %s", tokenPath)
		}
		return token, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("read auth token: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		return "", false, fmt.Errorf("create auth token dir: %w", err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", false, fmt.Errorf("generate auth token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		return "", false, fmt.Errorf("write auth token: %w", err)
	}

	return token, true, nil
}


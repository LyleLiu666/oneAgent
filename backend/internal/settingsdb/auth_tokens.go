package settingsdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type AuthToken struct {
	Token       string
	PrincipalID string
	CreatedAt   time.Time
	RevokedAt   *time.Time
}

func (d *DB) CreateAuthToken(ctx context.Context, principalID string) (AuthToken, error) {
	if d == nil || d.db == nil {
		return AuthToken{}, errors.New("settings db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return AuthToken{}, errors.New("principal_id is required")
	}

	token, err := generateToken(32)
	if err != nil {
		return AuthToken{}, err
	}

	now := nowMillis()
	_, err = d.db.ExecContext(ctx, `
INSERT INTO auth_tokens (token, principal_id, created_at_ms, revoked_at_ms)
VALUES (?, ?, ?, NULL);
`, token, principalID, now)
	if err != nil {
		return AuthToken{}, err
	}
	return AuthToken{
		Token:       token,
		PrincipalID: principalID,
		CreatedAt:   timeFromMillis(now),
	}, nil
}

func (d *DB) LookupAuthToken(ctx context.Context, token string) (string, bool, error) {
	if d == nil || d.db == nil {
		return "", false, errors.New("settings db is not initialized")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false, nil
	}

	var principalID string
	var revokedAt *int64
	err := d.db.QueryRowContext(ctx, `
SELECT principal_id, revoked_at_ms FROM auth_tokens WHERE token = ?;
`, token).Scan(&principalID, &revokedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	if revokedAt != nil && *revokedAt > 0 {
		return "", false, nil
	}
	return principalID, true, nil
}

func (d *DB) ListAuthTokens(ctx context.Context) ([]AuthToken, error) {
	if d == nil || d.db == nil {
		return nil, errors.New("settings db is not initialized")
	}

	rows, err := d.db.QueryContext(ctx, `
SELECT token, principal_id, created_at_ms, revoked_at_ms FROM auth_tokens ORDER BY created_at_ms DESC;
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuthToken
	for rows.Next() {
		var token, principalID string
		var createdAtMs int64
		var revokedAtMs *int64
		if err := rows.Scan(&token, &principalID, &createdAtMs, &revokedAtMs); err != nil {
			return nil, err
		}
		var revokedAt *time.Time
		if revokedAtMs != nil && *revokedAtMs > 0 {
			t := timeFromMillis(*revokedAtMs)
			revokedAt = &t
		}
		out = append(out, AuthToken{
			Token:       token,
			PrincipalID: principalID,
			CreatedAt:   timeFromMillis(createdAtMs),
			RevokedAt:   revokedAt,
		})
	}
	return out, rows.Err()
}

func (d *DB) RevokeAuthToken(ctx context.Context, token string) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token is required")
	}
	now := nowMillis()
	_, err := d.db.ExecContext(ctx, `
UPDATE auth_tokens SET revoked_at_ms = ? WHERE token = ?;
`, now, token)
	return err
}

func generateToken(n int) (string, error) {
	if n <= 0 {
		n = 32
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// sqlErrNoRows isolates sql.ErrNoRows without importing database/sql here (keeps dependency minimal).
var _ = sql.ErrNoRows

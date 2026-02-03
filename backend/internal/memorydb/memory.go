package memorydb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Entry struct {
	Seq         int64
	ID          string
	PrincipalID string
	CreatedAtMS int64
	Writer      string
	Type        string
	Workspace   string
	Title       string
	Content     string
	LinksJSON   string
	TagsJSON    string
}

type PullSyncResult struct {
	LastSyncedSeq    int64
	NewLastSyncedSeq int64
	TotalUnsynced    int
	Omitted          int
	Entries          []Entry
}

func (d *DB) AppendEntry(ctx context.Context, e Entry) (Entry, error) {
	if d == nil || d.db == nil {
		return Entry{}, errors.New("memory db is not initialized")
	}
	e.PrincipalID = strings.TrimSpace(e.PrincipalID)
	if e.PrincipalID == "" {
		return Entry{}, errors.New("principal_id is required")
	}
	e.Writer = strings.TrimSpace(e.Writer)
	if e.Writer == "" {
		return Entry{}, errors.New("writer is required")
	}
	e.Type = strings.TrimSpace(e.Type)
	if e.Type == "" {
		return Entry{}, errors.New("type is required")
	}
	if strings.TrimSpace(e.ID) == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAtMS <= 0 {
		e.CreatedAtMS = time.Now().UnixMilli()
	}

	res, err := d.db.ExecContext(ctx, `
INSERT INTO memory_entries (
  id, principal_id, created_at_ms, writer, type, workspace, title, content, links_json, tags_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`, e.ID, e.PrincipalID, e.CreatedAtMS, e.Writer, e.Type,
		nullIfEmpty(e.Workspace),
		nullIfEmpty(e.Title),
		nullIfEmpty(e.Content),
		nullIfEmpty(e.LinksJSON),
		nullIfEmpty(e.TagsJSON),
	)
	if err != nil {
		return Entry{}, err
	}
	seq, err := res.LastInsertId()
	if err != nil {
		return Entry{}, err
	}
	e.Seq = seq
	return e, nil
}

func (d *DB) QueryEntries(ctx context.Context, principalID string, sinceMS, untilMS int64, limit int) ([]Entry, error) {
	if d == nil || d.db == nil {
		return nil, errors.New("memory db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil, errors.New("principal_id is required")
	}
	if limit <= 0 {
		limit = 50
	}

	query := `
SELECT seq, id, principal_id, created_at_ms, writer, type, workspace, title, content, links_json, tags_json
FROM memory_entries
WHERE principal_id = ?
`
	args := []any{principalID}
	if sinceMS > 0 {
		query += " AND created_at_ms >= ?"
		args = append(args, sinceMS)
	}
	if untilMS > 0 {
		query += " AND created_at_ms <= ?"
		args = append(args, untilMS)
	}
	query += " ORDER BY created_at_ms DESC, seq DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Entry
	for rows.Next() {
		var e Entry
		var workspace, title, content, links, tags sql.NullString
		if err := rows.Scan(&e.Seq, &e.ID, &e.PrincipalID, &e.CreatedAtMS, &e.Writer, &e.Type, &workspace, &title, &content, &links, &tags); err != nil {
			return nil, err
		}
		e.Workspace = workspace.String
		e.Title = title.String
		e.Content = content.String
		e.LinksJSON = links.String
		e.TagsJSON = tags.String
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) PullSync(ctx context.Context, principalID, channel, peerWriter string, maxEntries int) (PullSyncResult, error) {
	if d == nil || d.db == nil {
		return PullSyncResult{}, errors.New("memory db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	channel = strings.TrimSpace(channel)
	peerWriter = strings.TrimSpace(peerWriter)
	if principalID == "" || channel == "" || peerWriter == "" {
		return PullSyncResult{}, errors.New("principal_id, channel, peer_writer are required")
	}
	if maxEntries <= 0 {
		maxEntries = 10
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return PullSyncResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	lastSynced := int64(0)
	err = tx.QueryRowContext(ctx, `
SELECT last_synced_seq
FROM memory_cursors
WHERE principal_id = ? AND channel = ? AND peer_writer = ?;
`, principalID, channel, peerWriter).Scan(&lastSynced)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PullSyncResult{}, err
	}

	var total sql.NullInt64
	var maxSeq sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*), MAX(seq)
FROM memory_entries
WHERE principal_id = ? AND writer = ? AND seq > ?;
`, principalID, peerWriter, lastSynced).Scan(&total, &maxSeq); err != nil {
		return PullSyncResult{}, err
	}
	if !total.Valid || total.Int64 == 0 {
		if err := tx.Commit(); err != nil {
			return PullSyncResult{}, err
		}
		return PullSyncResult{LastSyncedSeq: lastSynced, NewLastSyncedSeq: lastSynced, TotalUnsynced: 0, Omitted: 0, Entries: []Entry{}}, nil
	}

	rows, err := tx.QueryContext(ctx, `
SELECT seq, id, principal_id, created_at_ms, writer, type, workspace, title, content, links_json, tags_json
FROM memory_entries
WHERE principal_id = ? AND writer = ? AND seq > ?
ORDER BY seq DESC
LIMIT ?;
`, principalID, peerWriter, lastSynced, maxEntries)
	if err != nil {
		return PullSyncResult{}, err
	}

	var fetched []Entry
	for rows.Next() {
		var e Entry
		var workspace, title, content, links, tags sql.NullString
		if err := rows.Scan(&e.Seq, &e.ID, &e.PrincipalID, &e.CreatedAtMS, &e.Writer, &e.Type, &workspace, &title, &content, &links, &tags); err != nil {
			_ = rows.Close()
			return PullSyncResult{}, err
		}
		e.Workspace = workspace.String
		e.Title = title.String
		e.Content = content.String
		e.LinksJSON = links.String
		e.TagsJSON = tags.String
		fetched = append(fetched, e)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return PullSyncResult{}, err
	}
	_ = rows.Close()

	// Reverse to chronological order (best-effort).
	entries := make([]Entry, 0, len(fetched))
	for i := len(fetched) - 1; i >= 0; i-- {
		entries = append(entries, fetched[i])
	}

	newLastSynced := maxSeq.Int64
	if _, err := tx.ExecContext(ctx, `
INSERT INTO memory_cursors (principal_id, channel, peer_writer, last_synced_seq)
VALUES (?, ?, ?, ?)
ON CONFLICT(principal_id, channel, peer_writer) DO UPDATE SET
  last_synced_seq = excluded.last_synced_seq;
`, principalID, channel, peerWriter, newLastSynced); err != nil {
		return PullSyncResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return PullSyncResult{}, err
	}

	omitted := int(total.Int64) - len(entries)
	if omitted < 0 {
		omitted = 0
	}

	return PullSyncResult{
		LastSyncedSeq:    lastSynced,
		NewLastSyncedSeq: newLastSynced,
		TotalUnsynced:    int(total.Int64),
		Omitted:          omitted,
		Entries:          entries,
	}, nil
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

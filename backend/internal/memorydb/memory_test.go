package memorydb

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDB_AppendEntry_AndQueryByTimeWindow(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	principal := "p1"

	e1, err := db.AppendEntry(ctx, Entry{PrincipalID: principal, Writer: "SU", Type: "worklog", Content: "a", CreatedAtMS: 1000})
	if err != nil {
		t.Fatalf("append e1: %v", err)
	}
	_, _ = e1, err

	_, err = db.AppendEntry(ctx, Entry{PrincipalID: principal, Writer: "SU", Type: "worklog", Content: "b", CreatedAtMS: 2000})
	if err != nil {
		t.Fatalf("append e2: %v", err)
	}
	_, err = db.AppendEntry(ctx, Entry{PrincipalID: principal, Writer: "SW", Type: "findings", Content: "c", CreatedAtMS: 3000})
	if err != nil {
		t.Fatalf("append e3: %v", err)
	}

	got, err := db.QueryEntries(ctx, principal, 1500, 2500, 50)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Content != "b" || got[0].CreatedAtMS != 2000 {
		t.Fatalf("unexpected entry: %+v", got[0])
	}

	got, err = db.QueryEntries(ctx, principal, 0, 0, 10)
	if err != nil {
		t.Fatalf("query all: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	// DESC order
	if got[0].CreatedAtMS != 3000 || got[1].CreatedAtMS != 2000 || got[2].CreatedAtMS != 1000 {
		t.Fatalf("unexpected order: %+v", []int64{got[0].CreatedAtMS, got[1].CreatedAtMS, got[2].CreatedAtMS})
	}
}

func TestDB_PullSync_Last10AndOmittedCount(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "memory.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	principal := "p1"

	// 0 unsynced
	res, err := db.PullSync(ctx, principal, "SU", "SW", 10)
	if err != nil {
		t.Fatalf("pullsync empty: %v", err)
	}
	if res.TotalUnsynced != 0 || res.Omitted != 0 || len(res.Entries) != 0 {
		t.Fatalf("expected empty result, got %+v", res)
	}

	// 3 unsynced
	var lastSeq int64
	for i := 0; i < 3; i++ {
		e, err := db.AppendEntry(ctx, Entry{
			PrincipalID: principal,
			Writer:      "SW",
			Type:        "worklog",
			Content:     "m",
			CreatedAtMS: int64(1000 + i),
		})
		if err != nil {
			t.Fatalf("append: %v", err)
		}
		lastSeq = e.Seq
	}

	res, err = db.PullSync(ctx, principal, "SU", "SW", 10)
	if err != nil {
		t.Fatalf("pullsync 3: %v", err)
	}
	if res.TotalUnsynced != 3 || res.Omitted != 0 || len(res.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %+v", res)
	}
	if res.NewLastSyncedSeq != lastSeq {
		t.Fatalf("expected cursor to advance to %d, got %d", lastSeq, res.NewLastSyncedSeq)
	}

	res, err = db.PullSync(ctx, principal, "SU", "SW", 10)
	if err != nil {
		t.Fatalf("pullsync 3 again: %v", err)
	}
	if res.TotalUnsynced != 0 || len(res.Entries) != 0 {
		t.Fatalf("expected no new entries, got %+v", res)
	}

	// 23 unsynced
	for i := 0; i < 23; i++ {
		e, err := db.AppendEntry(ctx, Entry{
			PrincipalID: principal,
			Writer:      "SW",
			Type:        "worklog",
			Content:     "x",
			CreatedAtMS: int64(2000 + i),
		})
		if err != nil {
			t.Fatalf("append: %v", err)
		}
		lastSeq = e.Seq
	}

	res, err = db.PullSync(ctx, principal, "SU", "SW", 10)
	if err != nil {
		t.Fatalf("pullsync 23: %v", err)
	}
	if res.TotalUnsynced != 23 {
		t.Fatalf("expected total_unsynced=23, got %+v", res)
	}
	if res.Omitted != 13 {
		t.Fatalf("expected omitted=13, got %+v", res)
	}
	if len(res.Entries) != 10 {
		t.Fatalf("expected 10 entries, got %d", len(res.Entries))
	}
	if res.NewLastSyncedSeq != lastSeq {
		t.Fatalf("expected cursor to advance to %d, got %d", lastSeq, res.NewLastSyncedSeq)
	}
	// Ensure chronological order.
	for i := 1; i < len(res.Entries); i++ {
		if res.Entries[i].Seq <= res.Entries[i-1].Seq {
			t.Fatalf("expected ascending seq, got %d then %d", res.Entries[i-1].Seq, res.Entries[i].Seq)
		}
	}

	res, err = db.PullSync(ctx, principal, "SU", "SW", 10)
	if err != nil {
		t.Fatalf("pullsync after 23: %v", err)
	}
	if res.TotalUnsynced != 0 || len(res.Entries) != 0 {
		t.Fatalf("expected no new entries, got %+v", res)
	}
}


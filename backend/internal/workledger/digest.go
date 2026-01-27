package workledger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Digest struct {
	PrincipalID string    `json:"principal_id"`
	DayKey      string    `json:"day_key"`
	GeneratedAt time.Time `json:"generated_at"`

	Markdown string `json:"markdown"`
}

func (s *Store) DigestsDir(principalID string) string {
	return filepath.Join(s.baseDir, "digests", strings.TrimSpace(principalID))
}

func (s *Store) DigestPath(principalID, dayKey string) string {
	return filepath.Join(s.DigestsDir(principalID), strings.TrimSpace(dayKey)+".md")
}

func (s *Store) GenerateDigest(principalID string, day time.Time) (Digest, error) {
	if s == nil {
		return Digest{}, errors.New("store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return Digest{}, errors.New("principal_id is required")
	}

	dayKey := DayKey(day)
	receipts, err := s.listReceiptsForDay(principalID, day)
	if err != nil {
		return Digest{}, err
	}

	md := buildDigestMarkdown(principalID, dayKey, receipts)
	return Digest{
		PrincipalID: principalID,
		DayKey:      dayKey,
		GeneratedAt: time.Now().UTC(),
		Markdown:    md,
	}, nil
}

func (s *Store) GetOrCreateDigest(principalID string, day time.Time, refresh bool) (Digest, error) {
	if s == nil {
		return Digest{}, errors.New("store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return Digest{}, errors.New("principal_id is required")
	}

	dayKey := DayKey(day)
	path := s.DigestPath(principalID, dayKey)

	if !refresh {
		if b, err := os.ReadFile(path); err == nil {
			return Digest{
				PrincipalID: principalID,
				DayKey:      dayKey,
				GeneratedAt: time.Time{},
				Markdown:    string(b),
			}, nil
		}
	}

	d, err := s.GenerateDigest(principalID, day)
	if err != nil {
		return Digest{}, err
	}
	if err := writeTextAtomic(path, d.Markdown, 0o644); err != nil {
		return Digest{}, err
	}
	return d, nil
}

func (s *Store) listReceiptsForDay(principalID string, day time.Time) ([]Receipt, error) {
	receipts, err := s.ListReceipts(ListReceiptsQuery{
		PrincipalID: principalID,
		Limit:       1000,
	})
	if err != nil {
		return nil, err
	}

	start, end := dayBounds(day)
	out := make([]Receipt, 0, len(receipts))
	for _, r := range receipts {
		if r.FinishedAt.IsZero() {
			continue
		}
		f := r.FinishedAt.In(time.Local)
		if !f.Before(start) && f.Before(end) {
			out = append(out, r)
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].FinishedAt.After(out[j].FinishedAt) })
	return out, nil
}

func buildDigestMarkdown(principalID, dayKey string, receipts []Receipt) string {
	var succeeded []Receipt
	var failed []Receipt

	for _, r := range receipts {
		switch r.Status {
		case ReceiptStatusSucceeded:
			succeeded = append(succeeded, r)
		case ReceiptStatusFailed, ReceiptStatusTimedOut, ReceiptStatusInterrupted:
			failed = append(failed, r)
		default:
			// ignore canceled and unknown for v1 digest.
		}
	}

	var b strings.Builder
	b.WriteString("# Digest\n\n")
	b.WriteString("- principal_id: ")
	b.WriteString(principalID)
	b.WriteString("\n- day: ")
	b.WriteString(dayKey)
	b.WriteString("\n\n")

	b.WriteString("## Completed\n\n")
	if len(succeeded) == 0 {
		b.WriteString("- (none)\n\n")
	} else {
		for _, r := range succeeded {
			b.WriteString("- ")
			b.WriteString(oneLine(r.Summary))
			b.WriteString(" (receipt_id=")
			b.WriteString(r.ReceiptID)
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Needs Attention\n\n")
	if len(failed) == 0 {
		b.WriteString("- (none)\n\n")
	} else {
		for _, r := range failed {
			b.WriteString("- ")
			b.WriteString(oneLine(r.Summary))
			b.WriteString(" (status=")
			b.WriteString(string(r.Status))
			b.WriteString(", receipt_id=")
			b.WriteString(r.ReceiptID)
			b.WriteString(")\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Evidence\n\n")
	if len(receipts) == 0 {
		b.WriteString("- (no receipts)\n")
		return b.String()
	}

	for _, r := range receipts {
		if strings.TrimSpace(r.Artifacts.FindingsPath) == "" && strings.TrimSpace(r.Artifacts.TraceLogPath) == "" {
			continue
		}
		b.WriteString("- receipt_id=")
		b.WriteString(r.ReceiptID)
		b.WriteString("\n")
		if strings.TrimSpace(r.Artifacts.FindingsPath) != "" {
			b.WriteString("  - findings_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.FindingsPath))
			b.WriteString("\n")
		}
		if strings.TrimSpace(r.Artifacts.TraceLogPath) != "" {
			b.WriteString("  - trace_log_path: ")
			b.WriteString(strings.TrimSpace(r.Artifacts.TraceLogPath))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func DayKey(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.In(time.Local).Format("2006-01-02")
}

func dayBounds(t time.Time) (time.Time, time.Time) {
	local := t.In(time.Local)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	return start, start.Add(24 * time.Hour)
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}

func (s *Store) DeleteDigest(principalID, dayKey string) error {
	if s == nil {
		return errors.New("store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	dayKey = strings.TrimSpace(dayKey)
	if principalID == "" || dayKey == "" {
		return errors.New("principal_id and dayKey are required")
	}
	path := s.DigestPath(principalID, dayKey)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove digest: %w", err)
	}
	return nil
}


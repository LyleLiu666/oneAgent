package workledger

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DigestItem struct {
	ReceiptID     string       `json:"receipt_id"`
	Status        ReceiptStatus `json:"status"`
	WorkspaceRoot string       `json:"workspace_root,omitempty"`
	Kind          ReceiptKind   `json:"kind"`
	Summary       string       `json:"summary"`
	FinishedAt    time.Time    `json:"finished_at,omitempty"`

	Artifacts ReceiptArtifacts `json:"artifacts,omitempty"`
}

type DigestCluster struct {
	Key        string   `json:"key"`
	Count      int      `json:"count"`
	ReceiptIDs []string `json:"receipt_ids"`
}

type StructuredDigest struct {
	PrincipalID string    `json:"principal_id"`
	DayKey      string    `json:"day_key"`
	GeneratedAt time.Time `json:"generated_at"`

	Items    []DigestItem    `json:"items"`
	Clusters []DigestCluster `json:"clusters,omitempty"`
}

func (s *Store) DigestStructuredPath(principalID, dayKey string) string {
	return filepath.Join(s.DigestsDir(principalID), strings.TrimSpace(dayKey)+".json")
}

func (s *Store) GetStructuredDigest(principalID, dayKey string) (StructuredDigest, error) {
	if s == nil {
		return StructuredDigest{}, errors.New("store is nil")
	}
	principalID = strings.TrimSpace(principalID)
	dayKey = strings.TrimSpace(dayKey)
	if principalID == "" || dayKey == "" {
		return StructuredDigest{}, errors.New("principal_id and day_key are required")
	}

	path := s.DigestStructuredPath(principalID, dayKey)
	data, err := os.ReadFile(path)
	if err != nil {
		return StructuredDigest{}, err
	}

	var d StructuredDigest
	if err := json.Unmarshal(data, &d); err != nil {
		return StructuredDigest{}, err
	}
	return d, nil
}

func buildStructuredDigest(principalID, dayKey string, receipts []Receipt) StructuredDigest {
	items := make([]DigestItem, 0, len(receipts))
	for _, r := range receipts {
		items = append(items, DigestItem{
			ReceiptID:     r.ReceiptID,
			Status:        r.Status,
			WorkspaceRoot: strings.TrimSpace(r.WorkspaceRoot),
			Kind:          r.Kind,
			Summary:       oneLine(r.Summary),
			FinishedAt:    r.FinishedAt,
			Artifacts:     r.Artifacts,
		})
	}

	// Failure clustering v1: group by normalized summary (best-effort).
	clusterMap := map[string][]string{}
	for _, it := range items {
		switch it.Status {
		case ReceiptStatusFailed, ReceiptStatusTimedOut, ReceiptStatusInterrupted:
			key := strings.TrimSpace(it.Summary)
			if key == "" {
				key = "(no summary)"
			}
			if len(key) > 80 {
				key = key[:80] + "…"
			}
			clusterMap[key] = append(clusterMap[key], it.ReceiptID)
		}
	}

	var clusters []DigestCluster
	for key, ids := range clusterMap {
		if len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		clusters = append(clusters, DigestCluster{
			Key:        key,
			Count:      len(ids),
			ReceiptIDs: ids,
		})
	}
	sort.SliceStable(clusters, func(i, j int) bool {
		if clusters[i].Count != clusters[j].Count {
			return clusters[i].Count > clusters[j].Count
		}
		return clusters[i].Key < clusters[j].Key
	})

	return StructuredDigest{
		PrincipalID: principalID,
		DayKey:      dayKey,
		Items:       items,
		Clusters:    clusters,
	}
}

package benchmark

import (
	"os"
	"path/filepath"
	"testing"
)

func findRepoRootByDataset(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	start := dir
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "benchmarks", "datasets", "head2head_mvp.json")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	t.Fatalf("could not locate repo root with benchmarks/datasets/head2head_mvp.json from %s", start)
	return ""
}

func TestHead2HeadDataset_LoadAndValidate(t *testing.T) {
	repoRoot := findRepoRootByDataset(t)
	path := filepath.Join(repoRoot, "benchmarks", "datasets", "head2head_mvp.json")

	ds, err := LoadHead2HeadDataset(path)
	if err != nil {
		t.Fatalf("LoadHead2HeadDataset: %v", err)
	}
	if ds.ID == "" || ds.Version <= 0 {
		t.Fatalf("unexpected dataset meta: %+v", ds)
	}
	if len(ds.Cases) < 20 {
		t.Fatalf("expected >=20 cases, got %d", len(ds.Cases))
	}
}


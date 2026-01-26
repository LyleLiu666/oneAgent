package skill

import (
	"errors"
	"runtime"
	"testing"
)

func TestCheckEligibility_NoRequirements_IsEligible(t *testing.T) {
	got := CheckEligibility(Skill{ID: "a", Name: "a"}, nil)
	if !got.Eligible {
		t.Fatalf("expected eligible, got %+v", got)
	}
}

func TestCheckEligibility_MissingBin_IsIneligible(t *testing.T) {
	checker := NewBinaryChecker(func(name string) (string, error) {
		return "", errors.New("not found")
	})

	got := CheckEligibility(Skill{
		ID:   "a",
		Name: "a",
		Requires: &Requirements{
			Bins: []string{"memo"},
		},
	}, checker)

	if got.Eligible {
		t.Fatalf("expected ineligible, got %+v", got)
	}
	if got.Missing == nil || len(got.Missing.Bins) != 1 || got.Missing.Bins[0] != "memo" {
		t.Fatalf("unexpected missing: %+v", got.Missing)
	}
}

func TestCheckEligibility_AnyBins_SatisfiedByOne(t *testing.T) {
	checker := NewBinaryChecker(func(name string) (string, error) {
		if name == "rg" {
			return "/tmp/rg", nil
		}
		return "", errors.New("not found")
	})

	got := CheckEligibility(Skill{
		ID:   "a",
		Name: "a",
		Requires: &Requirements{
			AnyBins: []string{"rg", "grep"},
		},
	}, checker)

	if !got.Eligible {
		t.Fatalf("expected eligible, got %+v", got)
	}
}

func TestCheckEligibility_OSMismatch_IsIneligible(t *testing.T) {
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "linux"
	}
	got := CheckEligibility(Skill{
		ID:   "a",
		Name: "a",
		Requires: &Requirements{
			OS: []string{other},
		},
	}, nil)

	if got.Eligible {
		t.Fatalf("expected ineligible, got %+v", got)
	}
	if got.Missing == nil || len(got.Missing.OS) != 1 || got.Missing.OS[0] != other {
		t.Fatalf("unexpected missing.os: %+v", got.Missing)
	}
}

func TestFilterEligibleCatalog_FiltersSkills(t *testing.T) {
	checker := NewBinaryChecker(func(name string) (string, error) {
		if name == "memo" {
			return "/tmp/memo", nil
		}
		return "", errors.New("not found")
	})

	cat := &Catalog{
		Skills: []Skill{
			{ID: "ok", Name: "ok"},
			{ID: "need", Name: "need", Requires: &Requirements{Bins: []string{"memo"}}},
			{ID: "bad", Name: "bad", Requires: &Requirements{Bins: []string{"missing"}}},
		},
	}

	filtered := FilterEligibleCatalog(cat, checker)
	if filtered == nil || len(filtered.Skills) != 2 {
		t.Fatalf("expected 2 eligible skills, got %+v", filtered)
	}
	if filtered.Skills[0].ID != "ok" || filtered.Skills[1].ID != "need" {
		t.Fatalf("unexpected filtered order: %+v", filtered.Skills)
	}
}


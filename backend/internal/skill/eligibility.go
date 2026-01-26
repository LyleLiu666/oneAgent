package skill

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

type MissingRequirements struct {
	OS      []string `json:"os,omitempty"`
	Bins    []string `json:"bins,omitempty"`
	AnyBins []string `json:"any_bins,omitempty"`
	Env     []string `json:"env,omitempty"`
}

type Eligibility struct {
	Eligible bool                 `json:"eligible"`
	Missing  *MissingRequirements `json:"missing,omitempty"`
}

type LookPathFunc func(string) (string, error)

type BinaryChecker struct {
	lookPath LookPathFunc

	mu    sync.Mutex
	cache map[string]bool
}

func NewBinaryChecker(lookPath LookPathFunc) *BinaryChecker {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	return &BinaryChecker{
		lookPath: lookPath,
		cache:    make(map[string]bool, 32),
	}
}

func (c *BinaryChecker) Has(bin string) bool {
	if c == nil {
		c = NewBinaryChecker(nil)
	}
	bin = strings.TrimSpace(bin)
	if bin == "" {
		return false
	}

	c.mu.Lock()
	if ok, exists := c.cache[bin]; exists {
		c.mu.Unlock()
		return ok
	}
	c.mu.Unlock()

	_, err := c.lookPath(bin)
	ok := err == nil

	c.mu.Lock()
	c.cache[bin] = ok
	c.mu.Unlock()
	return ok
}

func CheckEligibility(s Skill, checker *BinaryChecker) Eligibility {
	req := normalizeRequirements(s.Requires)
	if req == nil {
		return Eligibility{Eligible: true}
	}

	missing := MissingRequirements{}

	if len(req.OS) > 0 {
		cur := runtime.GOOS
		supported := false
		for _, osName := range req.OS {
			if normalizeOS(osName) == cur {
				supported = true
				break
			}
		}
		if !supported {
			missing.OS = append(missing.OS, req.OS...)
		}
	}

	for _, bin := range req.Bins {
		if strings.TrimSpace(bin) == "" {
			continue
		}
		if checker.Has(bin) {
			continue
		}
		missing.Bins = append(missing.Bins, bin)
	}

	if len(req.AnyBins) > 0 {
		anyFound := false
		for _, bin := range req.AnyBins {
			if strings.TrimSpace(bin) == "" {
				continue
			}
			if checker.Has(bin) {
				anyFound = true
				break
			}
		}
		if !anyFound {
			missing.AnyBins = append(missing.AnyBins, req.AnyBins...)
		}
	}

	for _, env := range req.Env {
		key := strings.TrimSpace(env)
		if key == "" {
			continue
		}
		if strings.TrimSpace(os.Getenv(key)) != "" {
			continue
		}
		missing.Env = append(missing.Env, key)
	}

	ok := len(missing.OS) == 0 && len(missing.Bins) == 0 && len(missing.AnyBins) == 0 && len(missing.Env) == 0
	if ok {
		return Eligibility{Eligible: true}
	}
	return Eligibility{Eligible: false, Missing: &missing}
}

func FilterEligibleCatalog(catalog *Catalog, checker *BinaryChecker) *Catalog {
	if catalog == nil || len(catalog.Skills) == 0 {
		return catalog
	}
	if checker == nil {
		checker = NewBinaryChecker(nil)
	}

	out := &Catalog{
		Skills: make([]Skill, 0, len(catalog.Skills)),
		byID:   make(map[string]Skill),
		byName: make(map[string]Skill),
		byPath: make(map[string]Skill),
	}

	for _, s := range catalog.Skills {
		if !CheckEligibility(s, checker).Eligible {
			continue
		}
		out.Skills = append(out.Skills, s)
		out.byID[s.ID] = s
		out.byName[NormalizeName(s.Name)] = s
		out.byPath[s.Path] = s
	}

	return out
}


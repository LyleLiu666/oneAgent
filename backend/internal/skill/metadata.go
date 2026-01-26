package skill

import (
	"runtime"
	"sort"
	"strings"
)

// Requirements describes when a skill is runnable in the current environment.
// All fields are optional; empty requirements means "assumed eligible".
type Requirements struct {
	OS      []string `json:"os,omitempty"`
	Bins    []string `json:"bins,omitempty"`
	AnyBins []string `json:"any_bins,omitempty"`
	Env     []string `json:"env,omitempty"`
}

type InstallSpec struct {
	Kind    string   `json:"kind" yaml:"kind"`
	Label   string   `json:"label,omitempty" yaml:"label"`
	Formula string   `json:"formula,omitempty" yaml:"formula"`
	Module  string   `json:"module,omitempty" yaml:"module"`
	Package string   `json:"package,omitempty" yaml:"package"`
	URL     string   `json:"url,omitempty" yaml:"url"`
	Command string   `json:"command,omitempty" yaml:"command"`
	Bins    []string `json:"bins,omitempty" yaml:"bins"`
	OS      []string `json:"os,omitempty" yaml:"os"`
}

func normalizeRequirements(req *Requirements) *Requirements {
	if req == nil {
		return nil
	}

	out := &Requirements{
		OS:      normalizeOSList(req.OS),
		Bins:    normalizeStringList(req.Bins),
		AnyBins: normalizeStringList(req.AnyBins),
		Env:     normalizeEnvList(req.Env),
	}

	if len(out.OS) == 0 && len(out.Bins) == 0 && len(out.AnyBins) == 0 && len(out.Env) == 0 {
		return nil
	}
	return out
}

func normalizeInstallSpecs(specs []InstallSpec) []InstallSpec {
	out := make([]InstallSpec, 0, len(specs))
	for _, spec := range specs {
		kind := strings.ToLower(strings.TrimSpace(spec.Kind))
		if kind == "" {
			continue
		}
		item := InstallSpec{
			Kind:    kind,
			Label:   strings.TrimSpace(spec.Label),
			Formula: strings.TrimSpace(spec.Formula),
			Module:  strings.TrimSpace(spec.Module),
			Package: strings.TrimSpace(spec.Package),
			URL:     strings.TrimSpace(spec.URL),
			Command: strings.TrimSpace(spec.Command),
			Bins:    normalizeStringList(spec.Bins),
			OS:      normalizeOSList(spec.OS),
		}
		out = append(out, item)
	}
	return out
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

func normalizeEnvList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToUpper(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func normalizeOSList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = normalizeOS(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func normalizeOS(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "mac", "macos", "osx", "darwin":
		return "darwin"
	case "win", "win32", "windows":
		return "windows"
	case "linux":
		return "linux"
	default:
		return value
	}
}

func supportsCurrentOS(req *Requirements) bool {
	if req == nil || len(req.OS) == 0 {
		return true
	}
	cur := runtime.GOOS
	for _, osName := range req.OS {
		if normalizeOS(osName) == cur {
			return true
		}
	}
	return false
}


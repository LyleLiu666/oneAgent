package scope

import (
	"path"
	"path/filepath"
	"strings"
)

// Match reports whether name matches pattern.
//
// Supported meta characters:
// - "*" and "?" (same as path.Match)
// - "**" matches any number of segments
//
// Both inputs are treated as slash-separated paths.
func Match(pattern, name string) (bool, error) {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)

	pattern = strings.Trim(pattern, "/")
	name = strings.Trim(name, "/")

	patSegs := splitSegments(pattern)
	nameSegs := splitSegments(name)
	return matchSegments(patSegs, nameSegs)
}

// MatchAny reports whether name matches any pattern in patterns.
func MatchAny(patterns []string, name string) (bool, error) {
	name = filepath.ToSlash(strings.TrimSpace(name))
	name = strings.Trim(name, "/")

	for _, raw := range patterns {
		pat := filepath.ToSlash(strings.TrimSpace(raw))
		pat = strings.Trim(pat, "/")
		if pat == "" {
			continue
		}
		ok, err := Match(pat, name)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

func splitSegments(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, "/")
}

func matchSegments(pattern, name []string) (bool, error) {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			for len(pattern) > 1 && pattern[1] == "**" {
				pattern = pattern[1:]
			}
			if len(pattern) == 1 {
				return true, nil
			}
			for i := 0; i <= len(name); i++ {
				ok, err := matchSegments(pattern[1:], name[i:])
				if err != nil {
					return false, err
				}
				if ok {
					return true, nil
				}
			}
			return false, nil
		}

		if len(name) == 0 {
			return false, nil
		}

		matched, err := path.Match(pattern[0], name[0])
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
		pattern = pattern[1:]
		name = name[1:]
	}
	return len(name) == 0, nil
}


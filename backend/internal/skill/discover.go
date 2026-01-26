package skill

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type DiscoverOptions struct {
	WorkspaceRoot string
}

// Discover loads skills from configured sources and applies the precedence rule:
// .oneagent > .claude > .codex (dedup by normalized name).
func Discover(ctx context.Context, opts DiscoverOptions) (*Catalog, error) {
	sources := buildSources(opts.WorkspaceRoot)

	out := &Catalog{
		Skills: make([]Skill, 0, 64),
		byID:   make(map[string]Skill),
		byName: make(map[string]Skill),
		byPath: make(map[string]Skill),
	}

	for _, src := range sources {
		files, err := scanSkillFiles(ctx, src.Root)
		if err != nil {
			return nil, err
		}
		for _, skillPath := range files {
			s, err := parseSkill(skillPath, src.Source)
			if err != nil {
				continue
			}
			if s.ID == "" {
				continue
			}
			if _, exists := out.byID[s.ID]; exists {
				continue
			}
			out.Skills = append(out.Skills, s)
			out.byID[s.ID] = s
			out.byName[s.ID] = s
			out.byPath[s.Path] = s
		}
	}

	return out, nil
}

type sourceRoot struct {
	Source Source
	Root   string
}

func buildSources(workspaceRoot string) []sourceRoot {
	var sources []sourceRoot

	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot != "" {
		sources = append(sources, sourceRoot{
			Source: SourceOneAgent,
			Root:   filepath.Join(workspaceRoot, ".oneagent", "skills"),
		})
	}

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		sources = append(sources,
			sourceRoot{
				Source: SourceClaude,
				Root:   filepath.Join(home, ".claude", "skills"),
			},
			sourceRoot{
				Source: SourceCodex,
				Root:   filepath.Join(home, ".codex", "skills"),
			},
		)
	}

	return sources
}

func scanSkillFiles(ctx context.Context, root string) ([]string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}

	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	realRoot := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil && strings.TrimSpace(resolved) != "" {
		realRoot = resolved
	}

	queue := []string{realRoot}
	seen := map[string]struct{}{realRoot: {}}
	results := make([]string, 0, 64)

	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		dir := queue[0]
		queue = queue[1:]

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}

		for _, entry := range entries {
			name := entry.Name()
			full := filepath.Join(dir, name)

			if name == "SKILL.md" && !entry.IsDir() {
				results = append(results, full)
				continue
			}

			if entry.IsDir() {
				next := filepath.Clean(full)
				realNext := next
				if resolved, err := filepath.EvalSymlinks(next); err == nil && strings.TrimSpace(resolved) != "" {
					realNext = resolved
				}
				if _, ok := seen[realNext]; ok {
					continue
				}
				seen[realNext] = struct{}{}
				queue = append(queue, realNext)
				continue
			}

			if entry.Type()&os.ModeSymlink != 0 {
				resolved, err := filepath.EvalSymlinks(full)
				if err != nil || strings.TrimSpace(resolved) == "" {
					continue
				}
				stat, err := os.Stat(resolved)
				if err != nil || !stat.IsDir() {
					continue
				}
				real := filepath.Clean(resolved)
				if _, ok := seen[real]; ok {
					continue
				}
				seen[real] = struct{}{}
				queue = append(queue, real)
			}
		}
	}

	return results, nil
}

func parseSkill(path string, src Source) (Skill, error) {
	data, err := readSkillFile(path, 512*1024)
	if err != nil {
		return Skill{}, err
	}

	fm, rest, ok := parseFrontmatter(data)

	name := strings.TrimSpace(fm.Name)
	if name == "" {
		name = filepath.Base(filepath.Dir(path))
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Skill{}, errors.New("missing name")
	}

	desc := strings.TrimSpace(fm.Description)
	if desc == "" {
		desc = firstParagraph(string(rest), 200)
	}

	id := NormalizeName(name)
	if id == "" {
		return Skill{}, errors.New("invalid name")
	}

	s := Skill{
		ID:          id,
		Name:        name,
		Description: desc,
		Tags:        []string(fm.Tags),
		Keywords:    []string(fm.Keywords),
		Source:      src,
		Path:        path,
	}

	_ = ok // silence unused; keeps future extension obvious
	return s, nil
}

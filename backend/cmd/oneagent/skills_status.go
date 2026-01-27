package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/skill"
)

type skillStatusRow struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Source      string              `json:"source"`
	Path        string              `json:"path"`
	Requires    *skill.Requirements `json:"requires,omitempty"`
	Install     []skill.InstallSpec `json:"install,omitempty"`
	Eligibility skill.Eligibility   `json:"eligibility"`

	InstallHints []string `json:"install_hints,omitempty"`
}

func runSkillsStatus(args []string) {
	fs := flag.NewFlagSet("skills status", flag.ExitOnError)
	workspace := fs.String("workspace", "", "workspace root (optional; enables <workspace>/.oneagent/skills, <workspace>/skills, <workspace>/.claude/skills)")
	timeoutSeconds := fs.Int("timeout-seconds", 5, "timeout seconds (default: 5)")
	jsonOut := fs.Bool("json", false, "output json")
	_ = fs.Parse(args)

	workspaceRoot := strings.TrimSpace(*workspace)
	if workspaceRoot != "" {
		normalized, err := scope.NormalizeWorkspaceRoot(workspaceRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --workspace: %v\n", err)
			os.Exit(2)
		}
		workspaceRoot = normalized
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSeconds)*time.Second)
	defer cancel()

	manager := skill.NewManager(0)
	cat, err := manager.Load(ctx, workspaceRoot)
	if err != nil {
		log.Fatalf("load skills: %v", err)
	}
	if cat == nil || len(cat.Skills) == 0 {
		if *jsonOut {
			fmt.Println("[]")
			return
		}
		fmt.Println("No skills found.")
		return
	}

	checker := skill.NewBinaryChecker(nil)
	rows := make([]skillStatusRow, 0, len(cat.Skills))
	for _, s := range cat.Skills {
		elig := skill.CheckEligibility(s, checker)
		hints := buildInstallHints(s.Install, runtime.GOOS)

		rows = append(rows, skillStatusRow{
			ID:           s.ID,
			Name:         s.Name,
			Source:       string(s.Source),
			Path:         s.Path,
			Requires:     s.Requires,
			Install:      s.Install,
			Eligibility:  elig,
			InstallHints: hints,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Eligibility.Eligible != rows[j].Eligibility.Eligible {
			return rows[i].Eligibility.Eligible && !rows[j].Eligibility.Eligible
		}
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].ID < rows[j].ID
	})

	if *jsonOut {
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			log.Fatalf("marshal: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	printSkillStatusTable(rows)
}

func buildInstallHints(specs []skill.InstallSpec, goos string) []string {
	if len(specs) == 0 {
		return nil
	}
	goos = strings.ToLower(strings.TrimSpace(goos))

	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		if len(spec.OS) > 0 && !osListIncludes(spec.OS, goos) {
			continue
		}
		hint := renderInstallHint(spec)
		if hint == "" {
			continue
		}
		out = append(out, hint)
	}
	return out
}

func osListIncludes(values []string, goos string) bool {
	goos = strings.ToLower(strings.TrimSpace(goos))
	for _, v := range values {
		if strings.ToLower(strings.TrimSpace(v)) == goos {
			return true
		}
	}
	return false
}

func renderInstallHint(spec skill.InstallSpec) string {
	kind := strings.ToLower(strings.TrimSpace(spec.Kind))
	if kind == "" {
		return ""
	}

	label := strings.TrimSpace(spec.Label)
	prefix := ""
	if label != "" {
		prefix = label + ": "
	}

	switch kind {
	case "brew":
		if strings.TrimSpace(spec.Formula) == "" {
			return ""
		}
		return prefix + "brew install " + strings.TrimSpace(spec.Formula)
	case "go":
		module := strings.TrimSpace(spec.Module)
		if module == "" {
			return ""
		}
		if strings.Contains(module, "@") {
			return prefix + "go install " + module
		}
		return prefix + "go install " + module + "@latest"
	case "node":
		pkg := strings.TrimSpace(spec.Package)
		if pkg == "" {
			return ""
		}
		return prefix + "npm i -g " + pkg
	case "uv":
		pkg := strings.TrimSpace(spec.Package)
		if pkg == "" {
			return ""
		}
		return prefix + "uv tool install " + pkg
	case "download":
		url := strings.TrimSpace(spec.URL)
		if url == "" {
			return ""
		}
		return prefix + "download: " + url
	case "command":
		cmd := strings.TrimSpace(spec.Command)
		if cmd == "" {
			return ""
		}
		return prefix + cmd
	default:
		if strings.TrimSpace(spec.Command) != "" {
			return prefix + strings.TrimSpace(spec.Command)
		}
		return ""
	}
}

func printSkillStatusTable(rows []skillStatusRow) {
	eligible := 0
	for _, r := range rows {
		if r.Eligibility.Eligible {
			eligible++
		}
	}
	fmt.Printf("Skills: %d total (%d eligible, %d ineligible)\n\n", len(rows), eligible, len(rows)-eligible)

	for _, r := range rows {
		status := "✗"
		if r.Eligibility.Eligible {
			status = "✓"
		}

		name := strings.TrimSpace(r.Name)
		if name == "" {
			name = r.ID
		}

		fmt.Printf("%s %s (%s)\n", status, name, r.Source)
		if strings.TrimSpace(r.Path) != "" {
			fmt.Printf("  path: %s\n", r.Path)
		}

		if r.Eligibility.Eligible {
			continue
		}

		if r.Eligibility.Missing != nil {
			if len(r.Eligibility.Missing.OS) > 0 {
				fmt.Printf("  missing os: %s\n", strings.Join(r.Eligibility.Missing.OS, ", "))
			}
			if len(r.Eligibility.Missing.Bins) > 0 {
				fmt.Printf("  missing bins: %s\n", strings.Join(r.Eligibility.Missing.Bins, ", "))
			}
			if len(r.Eligibility.Missing.AnyBins) > 0 {
				fmt.Printf("  missing any_bins: %s\n", strings.Join(r.Eligibility.Missing.AnyBins, ", "))
			}
			if len(r.Eligibility.Missing.Env) > 0 {
				fmt.Printf("  missing env: %s\n", strings.Join(r.Eligibility.Missing.Env, ", "))
			}
		}

		if len(r.InstallHints) > 0 {
			fmt.Printf("  install:\n")
			for _, hint := range r.InstallHints {
				fmt.Printf("    - %s\n", hint)
			}
		}

		fmt.Println()
	}
}

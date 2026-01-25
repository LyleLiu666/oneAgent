package doctor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	stdruntime "runtime"
	"sort"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/buildinfo"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

type BinaryCheck struct {
	Name      string
	Available bool
	Path      string
	Required  bool
	Hint      string
}

type Report struct {
	Status           string
	Version          string
	Commit           string
	BuildDate        string
	OS               string
	Arch             string
	Profile          string
	AuthMode         string
	OneAgentHome     string
	SettingsDBPath   string
	AuthTokenPath    string
	DataDir          string
	LogsDir          string
	LogRetentionDays int
	Checks           []BinaryCheck
	Notes            []string
}

type LookPathFunc func(string) (string, error)

func Check(ctx context.Context, rt *runtime.Runtime, lookPath LookPathFunc) (Report, error) {
	if rt == nil || rt.Config == nil || rt.Layout == nil {
		return Report{}, errors.New("runtime is not initialized")
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	report := Report{
		Status:           "healthy",
		Version:          buildinfo.Version,
		Commit:           buildinfo.Commit,
		BuildDate:        buildinfo.Date,
		OS:               stdruntime.GOOS,
		Arch:             stdruntime.GOARCH,
		Profile:          rt.Config.Profile,
		AuthMode:         rt.Config.AuthMode,
		OneAgentHome:     rt.Layout.Home,
		SettingsDBPath:   rt.Layout.SettingsDBPath,
		AuthTokenPath:    rt.Layout.AuthTokenPath,
		DataDir:          rt.Layout.DataDir,
		LogsDir:          rt.Layout.LogsDir,
		LogRetentionDays: rt.Config.LogRetentionDays,
	}

	health, err := rt.Health(ctx)
	if err != nil {
		report.Status = "degraded"
		report.Notes = append(report.Notes, "runtime health check failed: "+err.Error())
	} else if health.Status != "" && health.Status != "healthy" {
		report.Status = health.Status
	}

	report.Checks = append(report.Checks,
		checkBinary(lookPath, "bash", true),
		checkBinary(lookPath, "git", true),
		checkBinary(lookPath, "jq", false),
		checkBinary(lookPath, "rg", false),
		checkBinary(lookPath, "pandoc", false),
		checkBinary(lookPath, "ffmpeg", false),
		checkBinary(lookPath, "wkhtmltopdf", false),
	)

	for i := range report.Checks {
		if report.Checks[i].Available {
			continue
		}
		if report.Checks[i].Name == "rg" {
			report.Notes = append(report.Notes, "`rg` not found: skills recall will fall back to `grep -R` (slower).")
		}
		if report.Checks[i].Required {
			report.Status = "degraded"
		}
	}

	sort.Slice(report.Checks, func(i, j int) bool {
		return report.Checks[i].Name < report.Checks[j].Name
	})

	return report, nil
}

func checkBinary(lookPath LookPathFunc, name string, required bool) BinaryCheck {
	path, err := lookPath(name)
	check := BinaryCheck{
		Name:      name,
		Available: err == nil,
		Path:      path,
		Required:  required,
		Hint:      installHint(name),
	}
	if check.Available {
		check.Hint = ""
	}
	return check
}

func installHint(binary string) string {
	osName := stdruntime.GOOS
	switch osName {
	case "darwin":
		return fmt.Sprintf("brew install %s", binary)
	case "linux":
		return fmt.Sprintf("install %s via your package manager (apt/yum/pacman)", binary)
	default:
		return fmt.Sprintf("install %s", binary)
	}
}

func Format(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "oneagent doctor (%s)\n", strings.TrimSpace(report.Version))
	fmt.Fprintf(&b, "build: commit=%s date=%s os=%s arch=%s\n", report.Commit, report.BuildDate, report.OS, report.Arch)
	fmt.Fprintf(&b, "profile=%s auth_mode=%s\n", report.Profile, report.AuthMode)
	fmt.Fprintf(&b, "home=%s\n", report.OneAgentHome)
	fmt.Fprintf(&b, "settings_db=%s\n", report.SettingsDBPath)
	fmt.Fprintf(&b, "auth_token_file=%s\n", report.AuthTokenPath)
	fmt.Fprintf(&b, "data_dir=%s\n", report.DataDir)
	fmt.Fprintf(&b, "logs_dir=%s (retention_days=%d)\n", report.LogsDir, report.LogRetentionDays)
	fmt.Fprintf(&b, "status=%s\n", report.Status)

	fmt.Fprintln(&b, "\nBinaries:")
	for _, check := range report.Checks {
		if check.Available {
			fmt.Fprintf(&b, "  - %s: OK (%s)\n", check.Name, check.Path)
			continue
		}
		hint := check.Hint
		if hint != "" {
			hint = " | " + hint
		}
		fmt.Fprintf(&b, "  - %s: MISSING%s\n", check.Name, hint)
	}

	if len(report.Notes) > 0 {
		fmt.Fprintln(&b, "\nNotes:")
		for _, note := range report.Notes {
			fmt.Fprintf(&b, "  - %s\n", note)
		}
	}

	return b.String()
}

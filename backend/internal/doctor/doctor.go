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
	"github.com/liu_y/oneAgent/backend/internal/netutil"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/shell"
)

type BinaryCheck struct {
	Name      string
	Available bool
	Path      string
	Source    string // system|bundled|explicit
	Required  bool
	Hint      string
}

type Report struct {
	Status                      string
	Version                     string
	Commit                      string
	BuildDate                   string
	OS                          string
	Arch                        string
	Profile                     string
	Bind                        string
	Port                        string
	AuthMode                    string
	OneAgentHome                string
	SettingsDBPath              string
	AuthTokenPath               string
	DataDir                     string
	LogsDir                     string
	LogRetentionDays            int
	FormalMemoryEnabled         bool
	FormalMemoryConnected       bool
	MemorySDKPreRecallPolicy    string
	MemorySDKToolsEnabled       bool
	MemorySDKTurnEndJobsEnabled bool
	BashSource                  string
	GitSource                   string
	Checks                      []BinaryCheck
	Notes                       []string
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
		Bind:             rt.Config.Bind,
		Port:             rt.Config.Port,
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
		report.FormalMemoryEnabled = health.FormalMemoryEnabled
		report.FormalMemoryConnected = health.FormalMemoryConnected
		report.MemorySDKPreRecallPolicy = health.MemorySDKPreRecallPolicy
		report.MemorySDKToolsEnabled = health.MemorySDKToolsEnabled
		report.MemorySDKTurnEndJobsEnabled = health.MemorySDKTurnEndJobsEnabled
	} else {
		report.FormalMemoryEnabled = health.FormalMemoryEnabled
		report.FormalMemoryConnected = health.FormalMemoryConnected
		report.MemorySDKPreRecallPolicy = health.MemorySDKPreRecallPolicy
		report.MemorySDKToolsEnabled = health.MemorySDKToolsEnabled
		report.MemorySDKTurnEndJobsEnabled = health.MemorySDKTurnEndJobsEnabled
	}
	if !netutil.IsLoopbackBind(rt.Config.Bind) {
		report.Notes = append(report.Notes, fmt.Sprintf("WARNING: bind=%s is non-loopback and may expose your local agent to the network (prefer bind=127.0.0.1).", rt.Config.Bind))
	}

	bashCheck := checkBinary(lookPath, "bash", true)
	gitCheck := checkBinary(lookPath, "git", true)

	if stdruntime.GOOS == "windows" {
		if resolved, err := shell.ResolveBashBinaryNoInstall(); err == nil && strings.TrimSpace(resolved.Path) != "" {
			bashCheck.Available = true
			bashCheck.Path = resolved.Path
			bashCheck.Source = string(resolved.Source)
			bashCheck.Hint = ""
			report.BashSource = string(resolved.Source)
		} else if shell.PortableGitArchivePath() != "" {
			bashCheck.Hint = "PortableGit archive detected; bash will be available after extraction"
		}

		if resolved, err := shell.ResolveGitBinaryNoInstall(); err == nil && strings.TrimSpace(resolved.Path) != "" {
			gitCheck.Available = true
			gitCheck.Path = resolved.Path
			gitCheck.Source = string(resolved.Source)
			gitCheck.Hint = ""
			report.GitSource = string(resolved.Source)
		} else if shell.PortableGitArchivePath() != "" {
			gitCheck.Hint = "PortableGit archive detected; git will be available after extraction"
		}
	} else {
		if bashCheck.Available {
			bashCheck.Source = "system"
			report.BashSource = "system"
		}
		if gitCheck.Available {
			gitCheck.Source = "system"
			report.GitSource = "system"
		}
	}

	report.Checks = append(report.Checks,
		bashCheck,
		gitCheck,
		checkBinary(lookPath, "gopls", false),
		checkBinary(lookPath, "typescript-language-server", false),
		checkBinary(lookPath, "jq", false),
		checkBinary(lookPath, "rg", false),
		checkBinary(lookPath, "pandoc", false),
		checkBinary(lookPath, "ffmpeg", false),
		checkBinary(lookPath, "wkhtmltopdf", false),
	)
	if stdruntime.GOOS == "darwin" {
		report.Checks = append(report.Checks, checkBinary(lookPath, "sandbox-exec", false))
	}

	for i := range report.Checks {
		if report.Checks[i].Available {
			continue
		}
		if report.Checks[i].Name == "rg" {
			report.Notes = append(report.Notes, "`rg` not found: skills recall and `rg` tool will fall back to a slower search backend (grep/go).")
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
	switch strings.TrimSpace(binary) {
	case "sandbox-exec":
		if stdruntime.GOOS == "darwin" {
			return "sandbox-exec is part of macOS (/usr/bin/sandbox-exec)"
		}
		return "native sandbox is not available on this OS"
	case "gopls":
		return "go install golang.org/x/tools/gopls@latest"
	case "typescript-language-server":
		return "npm i -g typescript typescript-language-server"
	case "pandoc":
		switch stdruntime.GOOS {
		case "windows":
			return "choco install pandoc (or winget install Pandoc)"
		case "darwin":
			return "brew install pandoc"
		case "linux":
			return "install pandoc via your package manager (apt/yum/pacman)"
		default:
			return "install pandoc"
		}
	}
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
	fmt.Fprintf(&b, "profile=%s listen=%s:%s auth_mode=%s\n", report.Profile, report.Bind, report.Port, report.AuthMode)
	fmt.Fprintf(&b, "home=%s\n", report.OneAgentHome)
	fmt.Fprintf(&b, "settings_db=%s\n", report.SettingsDBPath)
	fmt.Fprintf(&b, "auth_token_file=%s\n", report.AuthTokenPath)
	fmt.Fprintf(&b, "data_dir=%s\n", report.DataDir)
	fmt.Fprintf(&b, "logs_dir=%s (retention_days=%d)\n", report.LogsDir, report.LogRetentionDays)
	fmt.Fprintf(&b, "formalmemory_enabled=%t connected=%t prerecall_policy=%s\n", report.FormalMemoryEnabled, report.FormalMemoryConnected, strings.TrimSpace(report.MemorySDKPreRecallPolicy))
	fmt.Fprintf(&b, "formalmemory_tools_enabled=%t\n", report.MemorySDKToolsEnabled)
	fmt.Fprintf(&b, "formalmemory_turn_end_jobs_enabled=%t\n", report.MemorySDKTurnEndJobsEnabled)
	fmt.Fprintf(&b, "status=%s\n", report.Status)
	fmt.Fprintf(&b, "bash_source=%s\n", formatBinarySource(report.BashSource))
	fmt.Fprintf(&b, "git_source=%s\n", formatBinarySource(report.GitSource))

	fmt.Fprintln(&b, "\nBinaries:")
	for _, check := range report.Checks {
		if check.Available {
			src := ""
			if strings.TrimSpace(check.Source) != "" {
				src = " source=" + check.Source
			}
			fmt.Fprintf(&b, "  - %s: OK (%s)%s\n", check.Name, check.Path, src)
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

func formatBinarySource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "missing"
	}
	return source
}

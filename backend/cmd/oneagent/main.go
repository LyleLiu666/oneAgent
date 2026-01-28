package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	stdRuntime "runtime"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/buildinfo"
	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/doctor"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/server"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/skill"
	"github.com/liu_y/oneAgent/backend/internal/skillrecall"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	switch args[0] {
	case "--help", "-h", "help":
		printUsage()
		return
	case "--version", "version":
		fmt.Printf("oneagent %s (commit=%s date=%s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
		return
	case "serve":
		runServe(args[1:])
		return
	case "doctor":
		runDoctor(args[1:])
		return
	case "tokens":
		runTokens(args[1:])
		return
	case "skills":
		runSkills(args[1:])
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		os.Exit(2)
	}
}

type boolFlag struct {
	set   bool
	value bool
}

func (b *boolFlag) Set(v string) error {
	b.set = true
	b.value = strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
	return nil
}

func (b *boolFlag) String() string {
	if b == nil {
		return ""
	}
	if b.value {
		return "true"
	}
	return "false"
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	home := fs.String("home", "", "ONEAGENT_HOME (default: ~/.oneagent_default)")
	profile := fs.String("profile", "", "profile: local|dev (server is deprecated alias)")
	bind := fs.String("bind", "", "bind address (default: local=0.0.0.0, dev=127.0.0.1)")
	port := fs.String("port", "", "port (default: 8080)")
	openBrowser := fs.Bool("open", false, "open UI in default browser after server starts (best-effort)")
	defaultWorkspace := fs.String("workspace", "", "default workspace for UI auto-fill")
	authMode := fs.String("auth-mode", "", "auth mode: token|none (default: token)")
	bashRootDir := fs.String("bash-root-dir", "", "BASH_ROOT_DIR (default: ONEAGENT_HOME)")
	logRetentionDays := fs.Int("log-retention-days", 0, "log retention days (default: 30)")
	var enableTrace boolFlag
	fs.Var(&enableTrace, "enable-trace", "enable trace logging (true/false)")

	_ = fs.Parse(args)

	if strings.EqualFold(strings.TrimSpace(*profile), "server") {
		log.Printf("WARNING: profile=server is deprecated; using profile=local")
	}

	var enableTracePtr *bool
	if enableTrace.set {
		enableTracePtr = &enableTrace.value
	}

	cfg, err := config.Load(config.LoadOptions{
		Home:             *home,
		Profile:          *profile,
		Bind:             *bind,
		Port:             *port,
		DefaultWorkspace: *defaultWorkspace,
		AuthMode:         *authMode,
		EnableTrace:      enableTracePtr,
		BashRootDir:      *bashRootDir,
		LogRetentionDays: *logRetentionDays,
	})
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to init runtime: %v", err)
	}
	defer func() {
		_ = rt.Close()
	}()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(rt)
	}()

	if *openBrowser {
		baseURL := fmt.Sprintf("http://localhost:%s", cfg.Port)
		if err := waitForServerHealthy(baseURL, errCh, 5*time.Second); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
		if err := openURL(baseURL); err != nil {
			log.Printf("WARNING: failed to open browser: %v (open manually: %s)", err, baseURL)
		}
	}

	if err := <-errCh; err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	home := fs.String("home", "", "ONEAGENT_HOME (default: ~/.oneagent_default)")
	profile := fs.String("profile", "", "profile: local|dev (server is deprecated alias)")
	authMode := fs.String("auth-mode", "", "auth mode: token|none")
	var enableTrace boolFlag
	fs.Var(&enableTrace, "enable-trace", "enable trace logging (true/false)")
	logRetentionDays := fs.Int("log-retention-days", 0, "log retention days (default: 30)")

	_ = fs.Parse(args)

	var enableTracePtr *bool
	if enableTrace.set {
		enableTracePtr = &enableTrace.value
	}

	cfg, err := config.Load(config.LoadOptions{
		Home:             *home,
		Profile:          *profile,
		AuthMode:         *authMode,
		EnableTrace:      enableTracePtr,
		LogRetentionDays: *logRetentionDays,
	})
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to init runtime: %v", err)
	}
	defer func() {
		_ = rt.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	report, err := doctor.Check(ctx, rt, nil)
	if err != nil {
		log.Fatalf("doctor failed: %v", err)
	}

	fmt.Print(doctor.Format(report))
}

func printUsage() {
	fmt.Print(`oneagent - local tool runtime

Usage:
  oneagent serve [flags]    Start the server (UI + API)
  oneagent doctor [flags]   Run diagnostics
  oneagent tokens <cmd>     Manage local auth tokens
  oneagent skills search    Search skills (Top-K)
  oneagent --version        Print version

serve flags:
  --profile local|dev
  --home <path>
  --bind <addr>
  --port <port>
  --open
  --workspace <path>
  --auth-mode token|none
  --bash-root-dir <path>
  --log-retention-days <n>
  --enable-trace true|false
`)
}

func runTokens(args []string) {
	if len(args) == 0 {
		printTokensUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "create":
		runTokensCreate(args[1:])
	case "list":
		runTokensList(args[1:])
	case "revoke":
		runTokensRevoke(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown tokens command: %s\n\n", args[0])
		printTokensUsage()
		os.Exit(2)
	}
}

func openSettingsDBFromFlags(home, profile string) (*settingsdb.DB, func(), error) {
	cfg, err := config.Load(config.LoadOptions{
		Home:    home,
		Profile: profile,
		// Do not require auth settings to manage tokens on disk.
		AuthMode: "none",
	})
	if err != nil {
		return nil, nil, err
	}
	layout, err := runtime.EnsureLayout(cfg.Home)
	if err != nil {
		return nil, nil, err
	}
	db, err := settingsdb.Open(layout.SettingsDBPath)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = db.Close() }
	return db, cleanup, nil
}

func runTokensCreate(args []string) {
	fs := flag.NewFlagSet("tokens create", flag.ExitOnError)
	home := fs.String("home", "", "ONEAGENT_HOME (default: ~/.oneagent_default)")
	profile := fs.String("profile", "", "profile: local|dev")
	principal := fs.String("principal", "", "principal id (required)")
	_ = fs.Parse(args)

	if strings.TrimSpace(*principal) == "" {
		fmt.Fprintln(os.Stderr, "missing --principal")
		printTokensUsage()
		os.Exit(2)
	}

	db, cleanup, err := openSettingsDBFromFlags(*home, *profile)
	if err != nil {
		log.Fatalf("open settings db: %v", err)
	}
	defer cleanup()

	tok, err := db.CreateAuthToken(context.Background(), *principal)
	if err != nil {
		log.Fatalf("create token: %v", err)
	}

	out, _ := json.MarshalIndent(map[string]any{
		"token":        tok.Token,
		"principal_id": tok.PrincipalID,
		"created_at":   tok.CreatedAt.UTC().Format(time.RFC3339),
	}, "", "  ")
	fmt.Println(string(out))
}

func runTokensList(args []string) {
	fs := flag.NewFlagSet("tokens list", flag.ExitOnError)
	home := fs.String("home", "", "ONEAGENT_HOME (default: ~/.oneagent_default)")
	profile := fs.String("profile", "", "profile: local|dev")
	_ = fs.Parse(args)

	db, cleanup, err := openSettingsDBFromFlags(*home, *profile)
	if err != nil {
		log.Fatalf("open settings db: %v", err)
	}
	defer cleanup()

	list, err := db.ListAuthTokens(context.Background())
	if err != nil {
		log.Fatalf("list tokens: %v", err)
	}
	out, _ := json.MarshalIndent(list, "", "  ")
	fmt.Println(string(out))
}

func runTokensRevoke(args []string) {
	fs := flag.NewFlagSet("tokens revoke", flag.ExitOnError)
	home := fs.String("home", "", "ONEAGENT_HOME (default: ~/.oneagent_default)")
	profile := fs.String("profile", "", "profile: local|dev")
	token := fs.String("token", "", "token to revoke (required)")
	_ = fs.Parse(args)

	if strings.TrimSpace(*token) == "" {
		fmt.Fprintln(os.Stderr, "missing --token")
		printTokensUsage()
		os.Exit(2)
	}

	db, cleanup, err := openSettingsDBFromFlags(*home, *profile)
	if err != nil {
		log.Fatalf("open settings db: %v", err)
	}
	defer cleanup()

	if err := db.RevokeAuthToken(context.Background(), *token); err != nil {
		log.Fatalf("revoke token: %v", err)
	}
	fmt.Println(`{"ok":true}`)
}

func printTokensUsage() {
	fmt.Print(`oneagent tokens

Usage:
  oneagent tokens create --principal <id> [--home <dir>] [--profile local|dev]
  oneagent tokens list [--home <dir>] [--profile local|dev]
  oneagent tokens revoke --token <token> [--home <dir>] [--profile local|dev]
`)
}

func openURL(url string) error {
	switch stdRuntime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func waitForServerHealthy(baseURL string, errCh <-chan error, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()

	for {
		select {
		case err := <-errCh:
			return err
		case <-ticker.C:
			if checkServerHealthy(baseURL) {
				return nil
			}
		case <-deadline.C:
			return nil
		}
	}
}

func checkServerHealthy(baseURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func runSkills(args []string) {
	if len(args) == 0 {
		printSkillsUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "search":
		runSkillsSearch(args[1:])
	case "status", "check":
		runSkillsStatus(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown skills command: %s\n\n", args[0])
		printSkillsUsage()
		os.Exit(2)
	}
}

func runSkillsSearch(args []string) {
	fs := flag.NewFlagSet("skills search", flag.ExitOnError)
	query := fs.String("query", "", "search query (required)")
	workspace := fs.String("workspace", "", "workspace root (optional; enables <workspace>/.oneagent/skills, <workspace>/skills, <workspace>/.claude/skills)")
	limit := fs.Int("limit", 8, "max results (default: 8)")
	timeoutSeconds := fs.Int("timeout-seconds", 3, "timeout seconds (default: 3)")
	_ = fs.Parse(args)

	if strings.TrimSpace(*query) == "" {
		fmt.Fprintln(os.Stderr, "missing --query")
		printSkillsUsage()
		os.Exit(2)
	}

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

	res, err := skillrecall.Search(ctx, cat, *query, skillrecall.Options{MaxResults: *limit, Timeout: time.Duration(*timeoutSeconds) * time.Second}, nil)
	if err != nil {
		log.Fatalf("skills search: %v", err)
	}

	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	fmt.Println(string(data))
}

func printSkillsUsage() {
	fmt.Print(`oneagent skills

Usage:
  oneagent skills search --query "... " [--workspace <dir>] [--limit 8]
  oneagent skills status [--workspace <dir>] [--json]
  oneagent skills check  [--workspace <dir>] [--json]   # alias of status

Example:
  oneagent skills search --query "review this PR" --limit 8
`)
}

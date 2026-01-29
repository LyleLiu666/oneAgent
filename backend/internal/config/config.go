package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/scope"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
//
// Precedence (highest → lowest):
// 1) CLI flags (applied by caller via LoadOptions)
// 2) Environment variables
// 3) Config file at ONEAGENT_HOME/.oneagent/config/config.yaml
// 4) Defaults
type Config struct {
	Profile string

	Bind string
	Port string

	Home string
	// DefaultWorkspace is the server-provided default workspace for UI auto-fill.
	// It may be overridden by per-session workspace metadata (and client localStorage).
	DefaultWorkspace string

	AuthMode string

	// MCPAllowRemote enables remote (non-loopback) access to the MCP server endpoints.
	// Default is false (local-only).
	MCPAllowRemote bool

	EnableTrace bool

	BashRootDir string
	// BashRootDirExplicit indicates whether BashRootDir was explicitly configured
	// via flags/env/config file. When false, tools should default to workspace/home.
	BashRootDirExplicit bool

	LogRetentionDays int

	// Deprecated / unsupported (kept only to provide a clear error message if set).
	DatabaseURL string
}

type LoadOptions struct {
	Home             string
	Profile          string
	Bind             string
	Port             string
	DefaultWorkspace string

	AuthMode string

	EnableTrace *bool

	BashRootDir string

	LogRetentionDays int
}

var AppConfig *Config

func Load(opts LoadOptions) (*Config, error) {
	cfg := defaultConfig()

	home := strings.TrimSpace(opts.Home)
	if home == "" {
		home = strings.TrimSpace(os.Getenv("ONEAGENT_HOME"))
	}
	if home == "" {
		home = "~/.oneagent_default"
	}

	resolvedHome, err := expandPath(home)
	if err != nil {
		return nil, err
	}
	cfg.Home = resolvedHome

	// Load config file (if exists) before env/flags.
	if err := loadConfigFile(cfg); err != nil {
		return nil, err
	}

	// Env overrides.
	applyEnv(cfg)

	// Flags overrides.
	applyOptions(cfg, opts)

	normalize(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}

	AppConfig = cfg
	return cfg, nil
}

func GetConfig() *Config {
	if AppConfig != nil {
		return AppConfig
	}
	cfg, err := Load(LoadOptions{})
	if err != nil {
		// This should never happen with defaults.
		panic(err)
	}
	return cfg
}

func defaultConfig() *Config {
	return &Config{
		Profile:          "local",
		Bind:             "",
		Port:             "8080",
		AuthMode:         "token",
		EnableTrace:      false,
		BashRootDir:      "",
		LogRetentionDays: 30,
	}
}

type configFile struct {
	Profile          *string `yaml:"profile"`
	Bind             *string `yaml:"bind"`
	Port             *string `yaml:"port"`
	DefaultWorkspace *string `yaml:"default_workspace"`

	AuthMode *string `yaml:"auth_mode"`

	MCPAllowRemote *bool `yaml:"mcp_allow_remote"`

	EnableTrace *bool `yaml:"enable_trace"`

	BashRootDir *string `yaml:"bash_root_dir"`

	LogRetentionDays *int `yaml:"log_retention_days"`
}

func loadConfigFile(cfg *Config) error {
	path := filepath.Join(cfg.Home, ".oneagent", "config", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}

	var parsed configFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}

	if parsed.Profile != nil {
		cfg.Profile = *parsed.Profile
	}
	if parsed.Bind != nil {
		cfg.Bind = *parsed.Bind
	}
	if parsed.Port != nil {
		cfg.Port = *parsed.Port
	}
	if parsed.DefaultWorkspace != nil {
		cfg.DefaultWorkspace = *parsed.DefaultWorkspace
	}
	if parsed.AuthMode != nil {
		cfg.AuthMode = *parsed.AuthMode
	}
	if parsed.MCPAllowRemote != nil {
		cfg.MCPAllowRemote = *parsed.MCPAllowRemote
	}
	if parsed.EnableTrace != nil {
		cfg.EnableTrace = *parsed.EnableTrace
	}
	if parsed.BashRootDir != nil {
		cfg.BashRootDir = *parsed.BashRootDir
		if strings.TrimSpace(cfg.BashRootDir) != "" {
			cfg.BashRootDirExplicit = true
		}
	}
	if parsed.LogRetentionDays != nil {
		cfg.LogRetentionDays = *parsed.LogRetentionDays
	}

	return nil
}

func applyEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("PROFILE")); v != "" {
		cfg.Profile = v
	}
	if v := strings.TrimSpace(os.Getenv("BIND")); v != "" {
		cfg.Bind = v
	}
	if v := strings.TrimSpace(os.Getenv("PORT")); v != "" {
		cfg.Port = v
	}
	if v := strings.TrimSpace(os.Getenv("DEFAULT_WORKSPACE")); v != "" {
		cfg.DefaultWorkspace = v
	}
	if v := strings.TrimSpace(os.Getenv("MCP_ALLOW_REMOTE")); v != "" {
		cfg.MCPAllowRemote = parseBool(v)
	}
	if v := strings.TrimSpace(os.Getenv("AUTH_MODE")); v != "" {
		cfg.AuthMode = v
	}
	if v := strings.TrimSpace(os.Getenv("ENABLE_TRACE")); v != "" {
		cfg.EnableTrace = parseBool(v)
	}
	if v := strings.TrimSpace(os.Getenv("BASH_ROOT_DIR")); v != "" {
		cfg.BashRootDir = v
		cfg.BashRootDirExplicit = true
	}
	if v := strings.TrimSpace(os.Getenv("LOG_RETENTION_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.LogRetentionDays = n
		}
	}

	// Unsupported.
	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
}

func applyOptions(cfg *Config, opts LoadOptions) {
	if v := strings.TrimSpace(opts.Profile); v != "" {
		cfg.Profile = v
	}
	if v := strings.TrimSpace(opts.Bind); v != "" {
		cfg.Bind = v
	}
	if v := strings.TrimSpace(opts.Port); v != "" {
		cfg.Port = v
	}
	if v := strings.TrimSpace(opts.DefaultWorkspace); v != "" {
		cfg.DefaultWorkspace = v
	}
	if v := strings.TrimSpace(opts.AuthMode); v != "" {
		cfg.AuthMode = v
	}
	if opts.EnableTrace != nil {
		cfg.EnableTrace = *opts.EnableTrace
	}
	if v := strings.TrimSpace(opts.BashRootDir); v != "" {
		cfg.BashRootDir = v
		cfg.BashRootDirExplicit = true
	}
	if opts.LogRetentionDays != 0 {
		cfg.LogRetentionDays = opts.LogRetentionDays
	}
}

func normalize(cfg *Config) {
	cfg.Profile = strings.ToLower(strings.TrimSpace(cfg.Profile))
	cfg.AuthMode = strings.ToLower(strings.TrimSpace(cfg.AuthMode))
	cfg.Bind = strings.TrimSpace(cfg.Bind)
	cfg.Port = strings.TrimSpace(cfg.Port)
	cfg.BashRootDir = strings.TrimSpace(cfg.BashRootDir)
	cfg.DefaultWorkspace = strings.TrimSpace(cfg.DefaultWorkspace)

	if cfg.AuthMode == "password" {
		cfg.AuthMode = "token"
	}

	if cfg.Profile == "server" {
		// Deprecated alias; the CLI will print a warning. Keep behavior aligned to local.
		cfg.Profile = "local"
	}

	if cfg.Bind == "" {
		switch cfg.Profile {
		case "dev":
			cfg.Bind = "127.0.0.1"
		default:
			cfg.Bind = "0.0.0.0"
		}
	}

	if cfg.AuthMode == "" {
		cfg.AuthMode = "token"
	}

	if cfg.BashRootDirExplicit && cfg.BashRootDir != "" {
		expanded, err := expandPath(cfg.BashRootDir)
		if err == nil {
			cfg.BashRootDir = expanded
		}
	}

	if cfg.DefaultWorkspace != "" {
		expanded, err := expandPath(cfg.DefaultWorkspace)
		if err == nil {
			cfg.DefaultWorkspace = expanded
		}
	}

	if cfg.LogRetentionDays <= 0 {
		cfg.LogRetentionDays = 30
	}
}

func validate(cfg *Config) error {
	if cfg.DatabaseURL != "" {
		return fmt.Errorf("Postgres/DATABASE_URL is not supported in local tool mode")
	}
	if cfg.Home == "" {
		return errors.New("ONEAGENT_HOME is required")
	}
	if cfg.Profile != "local" && cfg.Profile != "dev" {
		return fmt.Errorf("invalid profile: %q (expected local|dev)", cfg.Profile)
	}
	if cfg.AuthMode != "token" && cfg.AuthMode != "none" {
		return fmt.Errorf("invalid AUTH_MODE: %q (expected token|none)", cfg.AuthMode)
	}
	if cfg.Port == "" {
		return errors.New("PORT is required")
	}
	if cfg.DefaultWorkspace != "" {
		normalized, err := scope.NormalizeWorkspaceRoot(cfg.DefaultWorkspace)
		if err != nil {
			return fmt.Errorf("invalid default workspace: %w", err)
		}
		cfg.DefaultWorkspace = normalized
	}
	return nil
}

func parseBool(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func expandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is empty")
	}

	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		if path == "~" {
			path = homeDir
		} else if strings.HasPrefix(path, "~/") {
			path = filepath.Join(homeDir, path[2:])
		} else {
			// "~user" not supported.
			return "", fmt.Errorf("unsupported path: %q", path)
		}
	}

	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("abs path: %w", err)
		}
		path = abs
	}

	path = filepath.Clean(path)
	return path, nil
}

// Now returns current time, overrideable in tests.
var Now = time.Now

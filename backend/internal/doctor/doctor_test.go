package doctor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestDoctor_ReportIncludesRgFallbackNote(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "token",
		LogRetentionDays: 1,
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	lookPath := func(name string) (string, error) {
		if name == "rg" {
			return "", errors.New("not found")
		}
		return "/usr/bin/" + name, nil
	}

	report, err := Check(context.Background(), rt, lookPath)
	if err != nil {
		t.Fatalf("doctor check: %v", err)
	}

	formatted := Format(report)
	if !strings.Contains(formatted, "rg: MISSING") {
		t.Fatalf("expected rg missing in report, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "listen=127.0.0.1:0") {
		t.Fatalf("expected listen address in report, got:\n%s", formatted)
	}
	if !strings.Contains(formatted, "skills recall and `rg` tool will fall back to a slower search backend (grep/go)") {
		t.Fatalf("expected fallback note, got:\n%s", formatted)
	}
	if strings.Contains(formatted, rt.AuthToken) {
		t.Fatalf("doctor output leaked auth token")
	}
}

func TestDoctor_ReportIncludesNetworkExposureWarning(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "0.0.0.0",
		Port:             "0",
		Home:             home,
		AuthMode:         "token",
		LogRetentionDays: 1,
	}

	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	lookPath := func(name string) (string, error) {
		return "/usr/bin/" + name, nil
	}

	report, err := Check(context.Background(), rt, lookPath)
	if err != nil {
		t.Fatalf("doctor check: %v", err)
	}

	formatted := Format(report)
	if !strings.Contains(formatted, "WARNING: bind=0.0.0.0") {
		t.Fatalf("expected network warning, got:\n%s", formatted)
	}
}

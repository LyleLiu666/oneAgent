package main

import (
	"reflect"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/skill"
)

func TestBuildInstallHints_FiltersByOS(t *testing.T) {
	specs := []skill.InstallSpec{
		{Kind: "brew", Formula: "rg", OS: []string{"darwin"}},
		{Kind: "brew", Formula: "ripgrep", OS: []string{"linux"}},
	}
	got := buildInstallHints(specs, "darwin")
	want := []string{"brew install rg"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRenderInstallHint_GoModuleAddsLatest(t *testing.T) {
	got := renderInstallHint(skill.InstallSpec{Kind: "go", Module: "example.com/tool"})
	want := "go install example.com/tool@latest"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderInstallHint_GoModuleKeepsVersion(t *testing.T) {
	got := renderInstallHint(skill.InstallSpec{Kind: "go", Module: "example.com/tool@v1.2.3"})
	want := "go install example.com/tool@v1.2.3"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderInstallHint_LabelPrefix(t *testing.T) {
	got := renderInstallHint(skill.InstallSpec{Kind: "brew", Label: "rg", Formula: "ripgrep"})
	want := "rg: brew install ripgrep"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

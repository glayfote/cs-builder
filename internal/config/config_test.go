package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_minimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cs-builder.yaml")
	content := `
version: 1
project_root: "."
scan_roots:
  - "2_if"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, used, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if used != path {
		t.Fatalf("used path: got %q want %q", used, path)
	}
	if cfg.Log.Directory != "logs" || cfg.Log.RetentionDays != 7 {
		t.Fatalf("defaults: %+v", cfg.Log)
	}
	wantExcl := []string{"bin", "obj", ".git", "node_modules"}
	if len(cfg.ScanExcludeDirNames) != len(wantExcl) {
		t.Fatalf("ScanExcludeDirNames: got %#v want %#v", cfg.ScanExcludeDirNames, wantExcl)
	}
	for i, w := range wantExcl {
		if cfg.ScanExcludeDirNames[i] != w {
			t.Fatalf("ScanExcludeDirNames[%d]: got %q want %q", i, cfg.ScanExcludeDirNames[i], w)
		}
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyDefaults_scanExcludeExplicitEmpty(t *testing.T) {
	cfg := &Config{ProjectRoot: ".", ScanRoots: []string{"x"}, ScanExcludeDirNames: []string{}}
	cfg.ApplyDefaults()
	if cfg.ScanExcludeDirNames == nil || len(cfg.ScanExcludeDirNames) != 0 {
		t.Fatalf("explicit empty exclude: got %#v", cfg.ScanExcludeDirNames)
	}
}

func TestValidate_errors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	_, _, err := Load(missing)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "read config") && !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("unexpected err: %v", err)
	}

	cfg := &Config{ProjectRoot: ".", ScanRoots: []string{"x"}}
	cfg.ApplyDefaults()
	cfg.Log.RetentionDays = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "retention_days") {
		t.Fatalf("want retention_days error, got %v", err)
	}

	cfg = &Config{Version: 99, ProjectRoot: ".", ScanRoots: []string{"x"}}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported config version") {
		t.Fatalf("want version error, got %v", err)
	}

	cfg = &Config{ProjectRoot: ".", ScanRoots: []string{"x"}, Artifacts: &ArtifactsConfig{CopyEnabled: true}}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "artifacts.destination") {
		t.Fatalf("want artifacts error, got %v", err)
	}
}

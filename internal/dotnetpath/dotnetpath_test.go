package dotnetpath

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolve_CS_BUILDER_DOTNET_missing(t *testing.T) {
	t.Setenv(EnvOverride, filepath.Join(t.TempDir(), "nonexistent.exe"))
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolve_CS_BUILDER_DOTNET_file(t *testing.T) {
	dir := t.TempDir()
	var want string
	if runtime.GOOS == "windows" {
		want = filepath.Join(dir, "dotnet.exe")
	} else {
		want = filepath.Join(dir, "dotnet")
	}
	if err := os.WriteFile(want, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvOverride, want)
	got, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

package sln

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCsprojPathsFromSolution_sample(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Caller")
	}
	slnPath := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "monorepo", "pfm", "3_common", "if", "if_a", "if_a.sln")
	slnPath = filepath.Clean(slnPath)
	got, err := CsprojPathsFromSolution(slnPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	want := filepath.Join(filepath.Dir(slnPath), "if_a.csproj")
	if filepath.Clean(got[0]) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got[0], want)
	}
}

func TestCsprojPathsFromSolution_missing(t *testing.T) {
	_, err := CsprojPathsFromSolution(filepath.Join(t.TempDir(), "none.sln"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCsprojPathsFromSolution_noProjects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.sln")
	if err := os.WriteFile(path, []byte(`
Microsoft Visual Studio Solution File, Format Version 12.00
Global
EndGlobal
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CsprojPathsFromSolution(path)
	if err == nil {
		t.Fatal("expected error")
	}
}

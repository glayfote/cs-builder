package buildorder

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"builder/cs-builder/internal/config"
)

func testMonorepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "monorepo"))
}

func TestResolve_pfm_pkg_b_closure(t *testing.T) {
	root := testMonorepoRoot(t)
	cfg := &config.Config{
		Version:     1,
		ProjectRoot: root,
		ScanRoots:   []string{"pfm"},
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	slnB := filepath.Join(root, "pfm", "4_driver", "pkg_b", "pkg_b.sln")
	order, err := Resolve(cfg, []string{slnB})
	if err != nil {
		t.Fatal(err)
	}
	if len(order) < 2 {
		t.Fatalf("order too short: %v", order)
	}
	// if_a は pkg_b の推移的依存の先頭付近にあるべき
	var ifAIdx, pkgBIdx int
	for i, p := range order {
		if strings.HasSuffix(filepath.ToSlash(p), "if_a/if_a.csproj") {
			ifAIdx = i
		}
		if strings.HasSuffix(filepath.ToSlash(p), "pkg_b/pkg_b.csproj") {
			pkgBIdx = i
		}
	}
	if ifAIdx >= pkgBIdx {
		t.Fatalf("if_a should build before pkg_b: order=%v", order)
	}
}

func TestResolve_cycle(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.csproj")
	b := filepath.Join(dir, "b.csproj")
	write := func(path, ref string) {
		body := `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup>`
		if ref != "" {
			rel, err := filepath.Rel(filepath.Dir(path), ref)
			if err != nil {
				t.Fatal(err)
			}
			body += `<ItemGroup><ProjectReference Include="` + filepath.ToSlash(rel) + `" /></ItemGroup>`
		}
		body += `</Project>`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(a, b)
	write(b, a)
	sln := filepath.Join(dir, "x.sln")
	slnBody := `
Microsoft Visual Studio Solution File, Format Version 12.00
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "a", "a.csproj", "{AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA}"
EndProject
`
	if err := os.WriteFile(sln, []byte(slnBody), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Version: 1, ProjectRoot: dir, ScanRoots: []string{"."}}
	cfg.ApplyDefaults()
	_, err := Resolve(cfg, []string{sln})
	if err == nil || !strings.Contains(err.Error(), "閉路") {
		t.Fatalf("want cycle error, got %v", err)
	}
}

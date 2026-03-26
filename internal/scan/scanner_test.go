package scan

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"builder/cs-builder/internal/config"
)

func testMonorepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	scanDir := filepath.Dir(file)
	return filepath.Clean(filepath.Join(scanDir, "..", "..", "testdata", "monorepo"))
}

func TestFindSolutions_layout(t *testing.T) {
	rootAbs, err := filepath.Abs(testMonorepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Version:     1,
		ProjectRoot: rootAbs,
		ScanRoots:   []string{"pfm/3_common", "pfm/4_driver"},
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	got, err := FindSolutions(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 {
		t.Fatalf("len=%d, got %+v", len(got), got)
	}
	var ifA, ifB, util1, util2, pkgA, pkgB bool
	for _, s := range got {
		switch {
		case s.ScanRoot == "pfm/3_common" && s.PackageDir == "if" && s.Tenant == "if_a":
			ifA = true
		case s.ScanRoot == "pfm/3_common" && s.PackageDir == "if" && s.Tenant == "if_b":
			ifB = true
		case s.ScanRoot == "pfm/3_common" && s.PackageDir == "utils" && s.Tenant == "util1":
			util1 = true
		case s.ScanRoot == "pfm/3_common" && s.PackageDir == "utils" && s.Tenant == "util2":
			util2 = true
		case s.ScanRoot == "pfm/4_driver" && s.PackageDir == "pkg_a" && s.Tenant == "":
			pkgA = true
		case s.ScanRoot == "pfm/4_driver" && s.PackageDir == "pkg_b" && s.Tenant == "":
			pkgB = true
		}
	}
	if !ifA || !ifB || !util1 || !util2 || !pkgA || !pkgB {
		t.Fatalf("flags ifA=%v ifB=%v util1=%v util2=%v pkgA=%v pkgB=%v solutions=%+v", ifA, ifB, util1, util2, pkgA, pkgB, got)
	}
}

func TestFindSolutions_excludesDirNames(t *testing.T) {
	root := t.TempDir()
	scan := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(scan, "okpkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scan, "okpkg", "App.sln"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(scan, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scan, "bin", "Skip.sln"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(scan, "pkg", "obj"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scan, "pkg", "obj", "Nested.sln"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:     1,
		ProjectRoot: root,
		ScanRoots:   []string{"src"},
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	got, err := FindSolutions(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d want 1, got %+v", len(got), got)
	}
	if got[0].PackageDir != "okpkg" {
		t.Fatalf("PackageDir=%q", got[0].PackageDir)
	}
}

func TestFindSolutions_recursiveDeepAndPrune(t *testing.T) {
	root := t.TempDir()
	// 深い階層の .sln（中間ディレクトリに .sln なし）
	deepDir := filepath.Join(root, "tree", "a", "b", "c")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deepDir, "Deep.sln"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	// 親に .sln があるときは子へ降りない（子の .sln は拾わない）
	if err := os.MkdirAll(filepath.Join(root, "tree", "parent", "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tree", "parent", "Root.sln"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tree", "parent", "child", "Hidden.sln"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:             1,
		ProjectRoot:         root,
		ScanRoots:           []string{"tree"},
		ScanExcludeDirNames: []string{}, // 非 nil の空配列 → 既定の bin/obj は入れない
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	got, err := FindSolutions(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2, got %+v", len(got), got)
	}
	var deep, rootOnly bool
	for _, s := range got {
		switch {
		case s.PackageDir == "a" && s.Tenant == "b/c":
			deep = true
		case s.PackageDir == "parent" && s.Tenant == "":
			rootOnly = true
		}
	}
	if !deep || !rootOnly {
		t.Fatalf("flags deep=%v parent=%v solutions=%+v", deep, rootOnly, got)
	}
}

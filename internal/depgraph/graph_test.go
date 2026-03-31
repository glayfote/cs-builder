package depgraph

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"builder/cs-builder/internal/scanner"
)

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller に失敗")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "monorepo", "pfm")
}

func scanAll(t *testing.T) []scanner.Solution {
	t.Helper()
	dir := testdataDir(t)
	solutions, err := scanner.ScanMultiple(dir, []string{"3_common", "4_driver"}, nil)
	if err != nil {
		t.Fatalf("ScanMultiple: %v", err)
	}
	return solutions
}

func findSolution(solutions []scanner.Solution, nameSubstr string) scanner.Solution {
	for _, s := range solutions {
		if strings.Contains(s.RelPath, nameSubstr) {
			return s
		}
	}
	return scanner.Solution{}
}

func TestBuild_NoWarnings(t *testing.T) {
	solutions := scanAll(t)
	_, warnings := Build(solutions)
	if len(warnings) > 0 {
		t.Errorf("warnings が発生: %v", warnings)
	}
}

func TestBuild_AssemblyMap(t *testing.T) {
	solutions := scanAll(t)
	g, _ := Build(solutions)

	cases := []struct {
		assembly string
		wantRel  string
	}{
		{"Pfm.Common.IfA", "if_a"},
		{"Pfm.Common.IfB", "if_b"},
		{"Pfm.Driver.PkgA", "pkg_a"},
	}

	for _, tc := range cases {
		t.Run(tc.assembly, func(t *testing.T) {
			node, ok := g.byAssembly[tc.assembly]
			if !ok {
				t.Fatalf("AssemblyName %q がグラフに存在しない", tc.assembly)
			}
			if !strings.Contains(node.Solution.RelPath, tc.wantRel) {
				t.Errorf("RelPath = %q, want contains %q", node.Solution.RelPath, tc.wantRel)
			}
		})
	}
}

func TestSort_NoDeps(t *testing.T) {
	solutions := scanAll(t)
	g, _ := Build(solutions)

	ifA := findSolution(solutions, filepath.Join("if", "if_a"))
	ifC := findSolution(solutions, filepath.Join("if", "if_c"))

	sorted, err := g.Sort([]scanner.Solution{ifC, ifA})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}

	if len(sorted) != 2 {
		t.Fatalf("len = %d, want 2", len(sorted))
	}
	for _, n := range sorted {
		if n.Level != 0 {
			t.Errorf("%s: Level = %d, want 0", n.AssemblyName, n.Level)
		}
	}
}

func TestSort_MultiLevel(t *testing.T) {
	solutions := scanAll(t)
	g, _ := Build(solutions)

	ifA := findSolution(solutions, filepath.Join("if", "if_a")+string(filepath.Separator))
	ifB := findSolution(solutions, filepath.Join("if", "if_b")+string(filepath.Separator))
	ifK := findSolution(solutions, filepath.Join("if", "if_k")+string(filepath.Separator))

	sorted, err := g.Sort([]scanner.Solution{ifK, ifA, ifB})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}

	levels := make(map[string]int)
	for _, n := range sorted {
		levels[n.AssemblyName] = n.Level
	}

	if levels["Pfm.Common.IfA"] >= levels["Pfm.Common.IfB"] {
		t.Errorf("if_a (L%d) は if_b (L%d) より前でなければならない",
			levels["Pfm.Common.IfA"], levels["Pfm.Common.IfB"])
	}
	if levels["Pfm.Common.IfB"] >= levels["Pfm.Common.IfK"] {
		t.Errorf("if_b (L%d) は if_k (L%d) より前でなければならない",
			levels["Pfm.Common.IfB"], levels["Pfm.Common.IfK"])
	}
}

func TestSort_FullGraph_PkgBLast(t *testing.T) {
	solutions := scanAll(t)
	g, _ := Build(solutions)

	sorted, err := g.Sort(solutions)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}

	if len(sorted) != len(solutions) {
		t.Fatalf("len = %d, want %d", len(sorted), len(solutions))
	}

	// pkg_b は pkg_a に依存するので、pkg_b のレベルは pkg_a より大きいはず
	var pkgALevel, pkgBLevel int
	for _, n := range sorted {
		if n.AssemblyName == "Pfm.Driver.PkgA" {
			pkgALevel = n.Level
		}
		if n.AssemblyName == "Pfm.Driver.PkgB" {
			pkgBLevel = n.Level
		}
	}
	if pkgALevel >= pkgBLevel {
		t.Errorf("pkg_a (L%d) は pkg_b (L%d) より前でなければならない", pkgALevel, pkgBLevel)
	}
}

func TestSort_SingleItem(t *testing.T) {
	solutions := scanAll(t)
	g, _ := Build(solutions)

	pkgA := findSolution(solutions, "pkg_a")
	sorted, err := g.Sort([]scanner.Solution{pkgA})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	if len(sorted) != 1 {
		t.Fatalf("len = %d, want 1", len(sorted))
	}
	if sorted[0].Level != 0 {
		t.Errorf("Level = %d, want 0 (単独選択時は依存先なし)", sorted[0].Level)
	}
}

func TestSort_DriverDependency(t *testing.T) {
	solutions := scanAll(t)
	g, _ := Build(solutions)

	pkgA := findSolution(solutions, "pkg_a")
	pkgB := findSolution(solutions, "pkg_b")
	ifA := findSolution(solutions, filepath.Join("if", "if_a")+string(filepath.Separator))

	sorted, err := g.Sort([]scanner.Solution{pkgB, pkgA, ifA})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}

	order := make(map[string]int)
	for i, n := range sorted {
		order[n.AssemblyName] = i
	}

	if order["Pfm.Driver.PkgA"] >= order["Pfm.Driver.PkgB"] {
		t.Errorf("pkg_a (pos %d) は pkg_b (pos %d) より前でなければならない",
			order["Pfm.Driver.PkgA"], order["Pfm.Driver.PkgB"])
	}
}

func TestExtractAssemblyFromHintPath(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`..\..\5_dll\common\if\if_a\Pfm.Common.IfA.dll`, "Pfm.Common.IfA"},
		{`../5_dll/driver/pkg_a/Pfm.Driver.PkgA.dll`, "Pfm.Driver.PkgA"},
		{"", ""},
		{`SomeLib.dll`, "SomeLib"},
	}
	for _, tc := range cases {
		got := extractAssemblyFromHintPath(tc.input)
		if got != tc.want {
			t.Errorf("extractAssemblyFromHintPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

package tui

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"builder/cs-builder/internal/builder"
	"builder/cs-builder/internal/depgraph"
	"builder/cs-builder/internal/scanner"
)

func testdataMonorepoPFM(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller に失敗")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "monorepo", "pfm")
}

func scanPFM(t *testing.T) []scanner.Solution {
	t.Helper()
	dir := testdataMonorepoPFM(t)
	solutions, err := scanner.ScanMultiple(dir, []string{"3_common", "4_driver"}, nil)
	if err != nil {
		t.Fatalf("ScanMultiple: %v", err)
	}
	return solutions
}

func findSln(solutions []scanner.Solution, nameSubstr string) scanner.Solution {
	for _, s := range solutions {
		if strings.Contains(s.RelPath, nameSubstr) {
			return s
		}
	}
	return scanner.Solution{}
}

func indexByAssembly(nodes []*depgraph.Node, asm string) int {
	for i, n := range nodes {
		if n.AssemblyName == asm {
			return i
		}
	}
	return -1
}

// pkg_b は pkg_a に依存する。最初の 1 スロットは常に pkg_a (高優先・唯一の準備完了のうちブロック解除側)。
func TestPickNextToStart_PrefersNeededByOthers(t *testing.T) {
	solutions := scanPFM(t)
	g, _ := depgraph.Build(solutions)
	pkgA := findSln(solutions, "pkg_a")
	pkgB := findSln(solutions, "pkg_b")
	if pkgA.AbsPath == "" || pkgB.AbsPath == "" {
		t.Fatal("pkg_a / pkg_b のソリューションが見つからない")
	}
	nodes, err := g.Sort([]scanner.Solution{pkgB, pkgA})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	m := newBuildModel(g, nodes, 1)
	got := m.pickNextToStart()
	if len(got) != 1 {
		t.Fatalf("pickNextToStart: len=%d, want 1", len(got))
	}
	if nodes[got[0]].AssemblyName != "pkg_a" {
		t.Errorf("first build = %q, want pkg_a", nodes[got[0]].AssemblyName)
	}
}

// pkg_a 完了後は pkg_b のみ準備完了。pkg_b は誰からも参照されないので低優先だがスロットは埋まる。
func TestPickNextToStart_AfterPrereqCompletes(t *testing.T) {
	solutions := scanPFM(t)
	g, _ := depgraph.Build(solutions)
	pkgA := findSln(solutions, "pkg_a")
	pkgB := findSln(solutions, "pkg_b")
	nodes, err := g.Sort([]scanner.Solution{pkgB, pkgA})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	m := newBuildModel(g, nodes, 1)
	iA := indexByAssembly(nodes, "pkg_a")
	iB := indexByAssembly(nodes, "pkg_b")
	if iA < 0 || iB < 0 {
		t.Fatalf("assembly index: pkg_a=%d pkg_b=%d", iA, iB)
	}
	m.items[iA].status = statusBuilding
	m.activeCount = 1
	m.completeItem(iA, builder.BuildResult{Success: true, Solution: m.items[iA].solution.AbsPath})
	got := m.pickNextToStart()
	if len(got) != 1 || got[0] != iB {
		t.Fatalf("after pkg_a: pickNextToStart = %v, want [%d] (pkg_b)", got, iB)
	}
}

// 準備完了が複数かつ高優先が混ざるとき、高優先を先に 1 本だけ起動する。
// if_a を選ばないことで pkg_a の選択内前提がなく、pkg_a と pkg_b が同時準備完了になる。
func TestPickNextToStart_HighBeforeLowWhenBothReady(t *testing.T) {
	solutions := scanPFM(t)
	g, _ := depgraph.Build(solutions)
	pkgA := findSln(solutions, "pkg_a")
	pkgB := findSln(solutions, "pkg_b")
	if pkgA.AbsPath == "" || pkgB.AbsPath == "" {
		t.Fatal("pkg_a / pkg_b が見つからない")
	}
	nodes, err := g.Sort([]scanner.Solution{pkgB, pkgA})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	m := newBuildModel(g, nodes, 1)
	got := m.pickNextToStart()
	if len(got) != 1 {
		t.Fatalf("pickNextToStart: len=%d, want 1", len(got))
	}
	if nodes[got[0]].AssemblyName != "pkg_a" {
		t.Errorf("first = %q, want pkg_a (pkg_b より被依存で高優先)", nodes[got[0]].AssemblyName)
	}
}

// 相互に内部依存がなく両方低優先のとき、RelPath 辞書順で先頭を選ぶ。
func TestPickNextToStart_TwoIndependentLowPriorityLexOrder(t *testing.T) {
	solutions := scanPFM(t)
	g, _ := depgraph.Build(solutions)
	ifA := findSln(solutions, string(filepath.Separator)+"if_a"+string(filepath.Separator))
	if ifA.AbsPath == "" {
		ifA = findSln(solutions, "if_a")
	}
	ifC := findSln(solutions, string(filepath.Separator)+"if_c"+string(filepath.Separator))
	if ifC.AbsPath == "" {
		ifC = findSln(solutions, "if_c")
	}
	if ifA.AbsPath == "" || ifC.AbsPath == "" {
		t.Fatal("if_a / if_c が見つからない")
	}
	nodes, err := g.Sort([]scanner.Solution{ifC, ifA})
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	m := newBuildModel(g, nodes, 1)
	got := m.pickNextToStart()
	if len(got) != 1 {
		t.Fatalf("pickNextToStart: len=%d, want 1", len(got))
	}
	want := 0
	if nodes[1].Solution.RelPath < nodes[0].Solution.RelPath {
		want = 1
	}
	if got[0] != want {
		t.Errorf("pick index %d, want %d (smaller RelPath %q vs %q)",
			got[0], want, nodes[0].Solution.RelPath, nodes[1].Solution.RelPath)
	}
}

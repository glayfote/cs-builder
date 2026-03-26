package buildorder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"builder/cs-builder/internal/config"
	"builder/cs-builder/internal/sln"
)

// Resolve はユーザーが選んだ .sln 群から、.csproj 間の依存を辿り、ビルドしてよい順序を返す。
//
// 依存辺の定義:
//   - ProjectReference: 参照先 .csproj は参照元より先にビルドする必要がある。
//   - HintPath の *.dll: ファイル名（拡張子除く）を AssemblyName とみなし、
//     assemblyCsprojIndex で得たプロジェクトへ同様の「先にビルド」辺を張る。
//
// アルゴリズムの概要:
//  1. 各 .sln からエントリ .csproj を列挙（シード）。
//  2. BFS で各 .csproj の依存を展開し、有向グラフ（prereq → dependent）を構築。
//  3. Kahn のアルゴリズムでトポロジカルソートし、閉路があればエラー。
//  4. シードおよびその推移的な前提だけを残し、全体のトポ順を保ったままフィルタする。
//
// 戻り値は絶対パスの .csproj 列で、dotnet build にその順で渡せる。
func Resolve(cfg *config.Config, selectedSlnPaths []string) ([]string, error) {
	if len(selectedSlnPaths) == 0 {
		return nil, fmt.Errorf("no solutions selected")
	}
	asmIdx, err := assemblyCsprojIndex(cfg)
	if err != nil {
		return nil, err
	}

	// ユーザーが UI で選んだ .sln に直接含まれる .csproj（ビルド対象の起点）。
	seeds := make(map[string]struct{})
	for _, sp := range selectedSlnPaths {
		sp = filepath.Clean(sp)
		projs, err := sln.CsprojPathsFromSolution(sp)
		if err != nil {
			return nil, err
		}
		for _, p := range projs {
			if err := fileMustExist(p, "solution project"); err != nil {
				return nil, err
			}
			seeds[p] = struct{}{}
		}
	}

	// キュー走査で辿り着いたすべての .csproj（依存展開済みノード集合）。
	expanded := make(map[string]struct{})
	// dependents[prereq] = prereq を前提とするプロジェクト集合。
	// 辺 prereq → dep は「prereq を先にビルドし、その後 dep」を意味する。
	dependents := make(map[string]map[string]struct{})
	addEdge := func(prereq, dependent string) {
		if prereq == dependent {
			return
		}
		if dependents[prereq] == nil {
			dependents[prereq] = make(map[string]struct{})
		}
		dependents[prereq][dependent] = struct{}{}
	}

	queue := make([]string, 0, len(seeds))
	for p := range seeds {
		queue = append(queue, p)
	}
	sort.Strings(queue)

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if _, done := expanded[p]; done {
			continue
		}
		expanded[p] = struct{}{}

		_, projDeps, dllHints, err := parseCsprojMeta(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}

		for _, d := range projDeps {
			if err := fileMustExist(d, "ProjectReference"); err != nil {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
			addEdge(d, p)
			if _, done := expanded[d]; !done {
				queue = append(queue, d)
			}
		}

		for _, base := range dllHints {
			target, ok := asmIdx[base]
			if !ok {
				// スキャン範囲外の DLL や、サードパーティのみの参照は辺なし（ビルド順に影響しない）。
				continue
			}
			if target == p {
				continue
			}
			addEdge(target, p)
			if _, done := expanded[target]; !done {
				queue = append(queue, target)
			}
		}
	}

	// グラフ上の全頂点（孤立した前提ノードも含む）。
	allNodes := make(map[string]struct{})
	for n := range expanded {
		allNodes[n] = struct{}{}
	}
	for from := range dependents {
		allNodes[from] = struct{}{}
		for to := range dependents[from] {
			allNodes[to] = struct{}{}
		}
	}

	// prereqCount[v] = v への入次数（= v が直接依存する前提プロジェクトの数）。
	prereqCount := make(map[string]int)
	for n := range allNodes {
		prereqCount[n] = 0
	}
	for _, toSet := range dependents {
		for to := range toSet {
			prereqCount[to]++
		}
	}

	// 入次数 0 から処理を始める（どのプロジェクトにも依存しない＝最初にビルド可能）。
	var ready []string
	for n := range allNodes {
		if prereqCount[n] == 0 {
			ready = append(ready, n)
		}
	}
	sort.Strings(ready)

	var order []string
	for len(ready) > 0 {
		n := ready[0]
		ready = ready[1:]
		order = append(order, n)
		for child := range dependents[n] {
			prereqCount[child]--
			if prereqCount[child] == 0 {
				ready = append(ready, child)
				sort.Strings(ready)
			}
		}
	}

	if len(order) != len(allNodes) {
		return nil, fmt.Errorf("依存に閉路があります（対象: %d プロジェクト）", len(allNodes))
	}

	// シードの推移的閉包だけ最終ビルド列に残す（同順序の部分列）。
	seedList := make([]string, 0, len(seeds))
	for p := range seeds {
		seedList = append(seedList, p)
	}
	sort.Strings(seedList)

	need := make(map[string]struct{})
	var addWithPrereqs func(string)
	addWithPrereqs = func(node string) {
		if _, ok := need[node]; ok {
			return
		}
		// node の各直接前提 prereq について、再帰的に need に入れる。
		for prereq, toSet := range dependents {
			if _, ok := toSet[node]; ok {
				addWithPrereqs(prereq)
			}
		}
		need[node] = struct{}{}
	}
	for _, s := range seedList {
		addWithPrereqs(s)
	}

	var filtered []string
	for _, n := range order {
		if _, ok := need[n]; ok {
			filtered = append(filtered, n)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("ビルド対象の .csproj が得られませんでした")
	}
	return filtered, nil
}

// fileMustExist は path が通常ファイルとして存在することを検証する（種別名 kind はエラーメッセージ用）。
func fileMustExist(path, kind string) error {
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q: %w", kind, path, err)
	}
	if st.IsDir() {
		return fmt.Errorf("%s %q is a directory", kind, path)
	}
	return nil
}

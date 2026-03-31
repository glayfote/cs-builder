package depgraph

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"builder/cs-builder/internal/artifact"
	"builder/cs-builder/internal/scanner"
)

// Node は DAG 内の 1 つのビルド対象（.sln）を表す。
type Node struct {
	Solution     scanner.Solution
	AssemblyName string
	Level        int // トポロジカルソートで算出される深さ (0 = 依存なし)
}

// Graph は Solution 間の依存 DAG を保持する。
type Graph struct {
	nodes      []*Node
	adj        map[string][]string // AssemblyName → 依存先 AssemblyNames
	byAssembly map[string]*Node    // AssemblyName → Node (逆引き)
}

// Nodes は全ノードを返す。
func (g *Graph) Nodes() []*Node { return g.nodes }

// InternalEdges はグラフ内に両端が存在するエッジのみを返す。
// キーは依存元の AssemblyName、値は依存先 AssemblyName のスライス。
// 外部ライブラリなどグラフ外への参照は除外される。
func (g *Graph) InternalEdges() map[string][]string {
	edges := make(map[string][]string)
	for name, deps := range g.adj {
		for _, dep := range deps {
			if _, ok := g.byAssembly[dep]; ok {
				edges[name] = append(edges[name], dep)
			}
		}
	}
	return edges
}

// Build は全ソリューションの .csproj をパースして依存グラフを構築する。
// パース失敗したソリューションは警告メッセージとともにスキップされ、
// グラフ構築自体は継続する。
func Build(solutions []scanner.Solution) (*Graph, []string) {
	g := &Graph{
		adj:        make(map[string][]string),
		byAssembly: make(map[string]*Node),
	}
	var warnings []string

	for _, sol := range solutions {
		csprojRel, err := artifact.ExtractCsprojPath(sol.AbsPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", sol.RelPath, err))
			continue
		}
		csprojPath := filepath.Join(filepath.Dir(sol.AbsPath), csprojRel)

		deps, err := parseCsprojDeps(csprojPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", sol.RelPath, err))
			continue
		}

		node := &Node{
			Solution:     sol,
			AssemblyName: deps.AssemblyName,
		}
		g.nodes = append(g.nodes, node)
		g.byAssembly[deps.AssemblyName] = node
		g.adj[deps.AssemblyName] = deps.Dependencies
	}

	return g, warnings
}

// Sort はユーザーが選択した solutions を依存解決順にソートして返す。
// 選択されていない依存先は自動追加しない。
// 選択サブセット内に循環がある場合はエラーを返す。
func (g *Graph) Sort(selected []scanner.Solution) ([]*Node, error) {
	selectedSet := make(map[string]bool, len(selected))
	for _, sol := range selected {
		if node, ok := g.nodeBySlnPath(sol.AbsPath); ok {
			selectedSet[node.AssemblyName] = true
		}
	}

	// 選択されたノード間のみでサブグラフを構築
	inDegree := make(map[string]int)
	subAdj := make(map[string][]string)
	for name := range selectedSet {
		inDegree[name] = 0
		subAdj[name] = nil
	}

	for name := range selectedSet {
		for _, dep := range g.adj[name] {
			if selectedSet[dep] {
				subAdj[dep] = append(subAdj[dep], name)
				inDegree[name]++
			}
		}
	}

	// Kahn のアルゴリズム (BFS) でトポロジカルソート + レベル算出
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	var result []*Node
	level := 0
	for len(queue) > 0 {
		var nextQueue []string
		for _, name := range queue {
			node := g.byAssembly[name]
			node.Level = level
			result = append(result, node)
			for _, dependent := range subAdj[name] {
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					nextQueue = append(nextQueue, dependent)
				}
			}
		}
		sort.Strings(nextQueue)
		queue = nextQueue
		level++
	}

	if len(result) != len(selectedSet) {
		var cycled []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycled = append(cycled, name)
			}
		}
		sort.Strings(cycled)
		return nil, fmt.Errorf("循環依存を検出: %s", strings.Join(cycled, ", "))
	}

	return result, nil
}

// nodeBySlnPath は .sln の絶対パスからノードを検索する。
func (g *Graph) nodeBySlnPath(absPath string) (*Node, bool) {
	for _, n := range g.nodes {
		if n.Solution.AbsPath == absPath {
			return n, true
		}
	}
	return nil, false
}

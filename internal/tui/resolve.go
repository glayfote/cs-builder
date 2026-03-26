package tui

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"builder/cs-builder/internal/scan"
)

// filterByTenant は tenant が TenantAll のときは all のコピー、それ以外は Tenant が一致する Solution だけを返す。
func filterByTenant(all []scan.Solution, tenant string) []scan.Solution {
	if tenant == TenantAll {
		out := make([]scan.Solution, len(all))
		copy(out, all)
		return out
	}
	var out []scan.Solution
	for _, s := range all {
		if s.Tenant == tenant {
			out = append(out, s)
		}
	}
	return out
}

// pickEntry はウィザードのチェックリスト 1 行を表す。
// Paths はその行を選んだときにビルド対象となる .sln の絶対パス（通常 1 要素。集約行では複数）。
type pickEntry struct {
	Label string
	Paths []string
}

// buildSlnPickEntries はテナントフィルタ後の各 Solution を 1 行ずつ並べる（Path 昇順）。
func buildSlnPickEntries(filtered []scan.Solution) []pickEntry {
	sols := append([]scan.Solution(nil), filtered...)
	sort.Slice(sols, func(i, j int) bool {
		return sols[i].Path < sols[j].Path
	})
	out := make([]pickEntry, 0, len(sols))
	for _, s := range sols {
		p := filepath.Clean(s.Path)
		out = append(out, pickEntry{
			Label: p,
			Paths: []string{p},
		})
	}
	return out
}

// pkgKey は (ScanRoot, PackageDir) でソリューションをグルーピングするためのキー。
type pkgKey struct {
	ScanRoot   string
	PackageDir string
}

// buildPackagePickEntries はフィルタ済み Solution を (ScanRoot, PackageDir) ごとにまとめ、
// 1 パッケージ＝1 行の pickEntry に変換する。同一グループ内の .sln パスはソートする。
func buildPackagePickEntries(filtered []scan.Solution) []pickEntry {
	m := make(map[pkgKey][]string)
	for _, s := range filtered {
		k := pkgKey{ScanRoot: s.ScanRoot, PackageDir: s.PackageDir}
		m[k] = append(m[k], s.Path)
	}
	keys := make([]pkgKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ScanRoot != keys[j].ScanRoot {
			return keys[i].ScanRoot < keys[j].ScanRoot
		}
		return keys[i].PackageDir < keys[j].PackageDir
	})
	out := make([]pickEntry, 0, len(keys))
	for _, k := range keys {
		paths := append([]string(nil), m[k]...)
		sort.Strings(paths)
		pd := k.PackageDir
		if pd == "" {
			pd = "(直下)"
		}
		out = append(out, pickEntry{
			Label: "[" + k.ScanRoot + "] " + pd,
			Paths: paths,
		})
	}
	return out
}

// folderKey はフォルダ単位選択のキー。PackageDir が空のときは「その scan_root 配下すべて」を意味する。
type folderKey struct {
	ScanRoot   string
	PackageDir string
}

// buildFolderPickEntries は scan_root 全体行と、PackageDir ごとの行を生成する。
// 各行の Paths は該当スコープに含まれる .sln パス（重複は uniqSortedPaths で除去）。
func buildFolderPickEntries(filtered []scan.Solution) []pickEntry {
	seen := make(map[folderKey]struct{})
	for _, s := range filtered {
		seen[folderKey{ScanRoot: s.ScanRoot, PackageDir: ""}] = struct{}{}
		if s.PackageDir != "" {
			seen[folderKey{ScanRoot: s.ScanRoot, PackageDir: s.PackageDir}] = struct{}{}
		}
	}
	keys := make([]folderKey, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ScanRoot != keys[j].ScanRoot {
			return keys[i].ScanRoot < keys[j].ScanRoot
		}
		return keys[i].PackageDir < keys[j].PackageDir
	})
	out := make([]pickEntry, 0, len(keys))
	for _, k := range keys {
		var label string
		if k.PackageDir == "" {
			label = "[" + k.ScanRoot + "] 配下すべて"
		} else {
			label = "[" + k.ScanRoot + "/" + k.PackageDir + "] このパッケージ配下すべて"
		}
		var paths []string
		for _, s := range filtered {
			if s.ScanRoot != k.ScanRoot {
				continue
			}
			if k.PackageDir == "" {
				paths = append(paths, s.Path)
				continue
			}
			if s.PackageDir == k.PackageDir {
				paths = append(paths, s.Path)
			}
		}
		sort.Strings(paths)
		paths = uniqSortedPaths(paths)
		out = append(out, pickEntry{Label: label, Paths: paths})
	}
	return out
}

// uniqSortedPaths はソート済みパスから隣接重複（Clean 後同一）を除いたスライスを返す。
func uniqSortedPaths(paths []string) []string {
	if len(paths) <= 1 {
		return paths
	}
	sort.Strings(paths)
	out := paths[:0]
	prev := ""
	for _, p := range paths {
		c := filepath.Clean(p)
		if c != prev {
			out = append(out, p)
			prev = c
		}
	}
	return out
}

// pathsFromPickSelection は選択された pickEntry インデックスに対応する Paths を結合し、重複を除去する。
func pathsFromPickSelection(entries []pickEntry, selected map[int]struct{}) []string {
	var acc []string
	for i := range selected {
		if i < 0 || i >= len(entries) {
			continue
		}
		acc = append(acc, entries[i].Paths...)
	}
	return uniqSortedPaths(acc)
}

// joinPathsPreview は paths を改行結合で表示用に整形する。max 件を超えたら「他 N 件」と省略する。
func joinPathsPreview(paths []string, max int) string {
	if len(paths) == 0 {
		return "(なし)"
	}
	if len(paths) <= max {
		return strings.Join(paths, "\n")
	}
	head := paths[:max]
	return strings.Join(head, "\n") + "\n… 他 " + strconv.Itoa(len(paths)-max) + " 件"
}

package tui

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"builder/cs-builder/internal/scan"
)

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

// pickEntry は手順 4 の一覧の 1 行。paths はその選択でビルド対象になる .sln（絶対パス）。
type pickEntry struct {
	Label string
	Paths []string
}

type pkgKey struct {
	ScanRoot   string
	PackageDir string
}

// buildPackagePickEntries は (ScanRoot, PackageDir) ごとにまとめた候補を返す。
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

type folderKey struct {
	ScanRoot   string
	PackageDir string // 空なら scan_root 全体
}

// buildFolderPickEntries は scan_root 全体、または scan_root 直下パッケージフォルダ単位の候補を返す。
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

// pathsFromPickSelection は選択した pickEntry の Paths を結合し重複除去する。
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

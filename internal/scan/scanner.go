// Package scan は cs-builder.yaml の scan_roots に基づき、ディスク上の .sln を探索する。
// 枝刈り規則（.sln があるディレクトリ以下は降りない、bin/obj 等はスキップ）を internal 側で一元管理する。
package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"builder/cs-builder/internal/config"
)

// Solution は探索で見つかった 1 つの .sln ファイルを表す。
type Solution struct {
	// Path は .sln ファイルへの絶対パス（クリーン済み）。
	Path string
	// ScanRoot はこの .sln が属する scan_roots のエントリ（設定ファイル上の相対パス、例: "2_if"）。
	ScanRoot string
	// PackageDir は .sln の親ディレクトリを scan_root からの相対パスで表したときの、先頭セグメント（例: "logger", "sql"）。
	// .sln が scan_root 直下にある場合は空文字。
	PackageDir string
	// Tenant は上記相対パスで 2 セグメント以上あるとき、2 セグメント目以降を "/" で結合したもの（例: "tenant1"、深い場合は "a/b"）。
	// 1 セグメントのみのときは空文字。
	Tenant string
}

// FindSolutions は各 scan_root 配下を再帰的に走査し *.sln を列挙する。
//
// 方針（除外ベース）:
//   - project_root + scan_roots[i] をツリーの根とする。
//   - ディレクトリを深さ優先で辿るが、子ディレクトリ名が cfg.ScanExcludeDirNames に一致するもの
//     （Trim 後・セグメント一致・大文字小文字無視）は入らない。
//   - あるディレクトリの直下に .sln が 1 つでもあれば、それらをすべて収集し、そのディレクトリ以下には
//     降りない（同一ツリーに .sln があるフォルダの子に .sln は無い前提での枝刈り）。
//   - 直下に .sln が無いディレクトリのみ、除外に該当しない子ディレクトリへ再帰する。
//
// PackageDir / Tenant は .sln の親ディレクトリについて、scan_root からの相対パスをセグメント分割して付与する。
// 同一 .sln は Path（クリーン済み）で 1 回だけ結果に含める。戻り値は Path の昇順。
func FindSolutions(cfg *config.Config) ([]Solution, error) {
	root, err := cfg.ResolvedProjectRoot()
	if err != nil {
		return nil, err
	}
	var out []Solution
	// 複数 scan_root や同一ファイルの重複列挙を防ぐ（キーはクリーン済み絶対パス）。
	seen := make(map[string]struct{})
	excl := scanExcludeNameSet(cfg)

	for _, scanRel := range cfg.ScanRoots {
		scanRel = strings.TrimSpace(scanRel)
		scanAbs := filepath.Join(root, filepath.FromSlash(scanRel))
		// 各ルートは独立したサブツリーとして深さ優先走査する。
		if err := walkScanTree(scanAbs, scanAbs, scanRel, excl, seen, &out); err != nil {
			return nil, err
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// walkScanTree は scanAbs を根とするサブツリー内を探索する。dir は現在のディレクトリ（絶対パス）。
// 直下に .sln があればそのディレクトリは「末端」とみなし子へは進まない。
func walkScanTree(scanAbs, dir, scanRel string, excl map[string]struct{}, seen map[string]struct{}, out *[]Solution) error {
	slns, err := slnFilesInDir(dir)
	if err != nil {
		if dir == scanAbs {
			return fmt.Errorf("read scan root %q: %w", scanRel, err)
		}
		return err
	}
	if len(slns) > 0 {
		for _, p := range slns {
			if !addUnique(seen, p) {
				continue
			}
			// TUI のパッケージ／テナント表示用に、scan_root から .sln 親フォルダまでの相対パスを分解する。
			pkgDir, tenant := solutionLabelsUnderScanRoot(scanAbs, filepath.Dir(p))
			*out = append(*out, Solution{
				Path:       p,
				ScanRoot:   scanRel,
				PackageDir: pkgDir,
				Tenant:     tenant,
			})
		}
		// 子ディレクトリに別の .sln があっても探索しない（モノレポ運用の前提）。
		return nil
	}

	ents, err := os.ReadDir(dir)
	if err != nil {
		if dir == scanAbs {
			return fmt.Errorf("read scan root %q: %w", scanRel, err)
		}
		return fmt.Errorf("read dir %q: %w", dir, err)
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if scanExcludeName(excl, e.Name()) {
			continue
		}
		child := filepath.Join(dir, e.Name())
		if err := walkScanTree(scanAbs, child, scanRel, excl, seen, out); err != nil {
			return err
		}
	}
	return nil
}

// solutionLabelsUnderScanRoot は .sln の親ディレクトリ slnParentDir について、scanAbs からの相対パスから PackageDir / Tenant を求める。
// 第 1 パスセグメントを PackageDir、残りを "/" 結合した Tenant とする（1 セグメントのみなら Tenant は空）。
func solutionLabelsUnderScanRoot(scanAbs, slnParentDir string) (packageDir, tenant string) {
	rel, err := filepath.Rel(scanAbs, slnParentDir)
	if err != nil {
		return "", ""
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return "", ""
	}
	segs := strings.Split(filepath.ToSlash(rel), "/")
	var parts []string
	for _, s := range segs {
		if s != "" && s != "." {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], "/")
}

// scanExcludeNameSet は設定の除外名からルックアップ用セットを作る。キーは小文字。
func scanExcludeNameSet(cfg *config.Config) map[string]struct{} {
	m := make(map[string]struct{})
	for _, raw := range cfg.ScanExcludeDirNames {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		m[strings.ToLower(s)] = struct{}{}
	}
	return m
}

// scanExcludeName はディレクトリ名が除外セットに含まれるかを返す（大文字小文字無視）。
func scanExcludeName(set map[string]struct{}, dirName string) bool {
	_, ok := set[strings.ToLower(dirName)]
	return ok
}

// slnFilesInDir は dir の直下にあるファイルのうち、拡張子が .sln のものだけを絶対パスで返す。
// サブディレクトリ内は見ない。拡張子の大文字小文字は区別しない（.SLN も対象）。
// 返すパスはファイル名の辞書順でソートされる。
func slnFilesInDir(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}
	var paths []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.EqualFold(filepath.Ext(name), ".sln") {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// addUnique は path を filepath.Clean したキーで seen に未登録なら登録し true を返す。
// 既に登録済みなら false（呼び出し側で結果スライスへの追加をスキップする）。
func addUnique(seen map[string]struct{}, path string) bool {
	clean := filepath.Clean(path)
	if _, ok := seen[clean]; ok {
		return false
	}
	seen[clean] = struct{}{}
	return true
}

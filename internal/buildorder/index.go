package buildorder

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"builder/cs-builder/internal/config"
)

// assemblyCsprojIndex は cfg.ScanRoots それぞれの配下を filepath.WalkDir で走査し、
// 「AssemblyName → その .csproj の絶対パス」のマップを構築する。
//
// scan_roots 外のプロジェクトはインデックスに載らないため、HintPath の DLL が
// スキャン範囲外だけで解決される場合は依存辺が張れない。
//
// 同一 AssemblyName が 2 つの .csproj で使われている場合はエラーを返す。
func assemblyCsprojIndex(cfg *config.Config) (map[string]string, error) {
	root, err := cfg.ResolvedProjectRoot()
	if err != nil {
		return nil, err
	}
	excl := scanExcludeSet(cfg)
	idx := make(map[string]string)
	for _, scanRel := range cfg.ScanRoots {
		scanRel = strings.TrimSpace(scanRel)
		if scanRel == "" {
			continue
		}
		scanAbs := filepath.Join(root, filepath.FromSlash(scanRel))
		if err := filepath.WalkDir(scanAbs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// bin / obj 等は設定の除外名に従いサブツリーごとスキップする。
				if scanExcludeName(excl, d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".csproj") {
				return nil
			}
			asm, _, _, err := parseCsprojMeta(path)
			if err != nil {
				return err
			}
			if asm == "" {
				return nil
			}
			if prev, ok := idx[asm]; ok && prev != path {
				return fmt.Errorf("duplicate AssemblyName %q: %q and %q", asm, prev, path)
			}
			idx[asm] = path
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return idx, nil
}

// scanExcludeSet は cfg.ScanExcludeDirNames からディレクトリ名ルックアップ用のセットを作る（キーは小文字）。
func scanExcludeSet(cfg *config.Config) map[string]struct{} {
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

// scanExcludeName は dirName が除外セットに含まれるかを返す（大文字小文字無視）。
func scanExcludeName(set map[string]struct{}, dirName string) bool {
	_, ok := set[strings.ToLower(dirName)]
	return ok
}

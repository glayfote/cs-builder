// Package scanner は指定ディレクトリ配下の C# ソリューションファイル (.sln) を
// 再帰的に探索する機能を提供する。
//
// ビルド成果物ディレクトリ (bin, obj) やバージョン管理ディレクトリ (.git) など
// 探索不要なディレクトリは自動的にスキップされる。
package scanner

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Solution は探索で発見された単一の .sln ファイルの情報を保持する。
// TUI の選択リストでの表示名として RelPath を、
// ビルド実行時のパス解決として AbsPath を使用する。
type Solution struct {
	Name    string // ソリューション名 (拡張子を除いたファイル名, 例: "if_a")
	AbsPath string // ファイルシステム上の絶対パス
	RelPath string // baseDir を起点とした相対パス (TUI での表示用)
}

// defaultExcludes はハードコードされた除外ディレクトリ名。
// extraExcludes の有無に関わらず常に除外される。
var defaultExcludes = map[string]bool{
	"bin":          true,
	"obj":          true,
	".git":         true,
	"node_modules": true,
}

// Scan は baseDir 配下を再帰的に走査し、見つかった .sln ファイルの一覧を返す。
//
// 走査時に以下のディレクトリは常に除外される:
//   - bin, obj : .NET ビルド成果物ディレクトリ
//   - .git     : Git リポジトリメタデータ
//   - node_modules : Node.js 依存パッケージ
//
// extraExcludes でユーザ定義の除外パターンを追加指定できる。
// サポートするパターン形式:
//   - "dirname"      : ディレクトリ名の完全一致 (例: "packages")
//   - "**/dirname"   : 任意の深さの dirname にマッチ (dirname 完全一致と同義)
//   - "prefix/**"    : prefix で始まるパスの配下を丸ごと除外
//   - "pattern"      : filepath.Match 互換のワイルドカード
//
// 拡張子の比較は大文字小文字を無視する (例: .SLN も検出対象)。
// 個別ファイルの読み取りエラーはスキップし、走査を継続する。
func Scan(baseDir string, extraExcludes []string) ([]Solution, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}

	matcher := newExcludeMatcher(extraExcludes)

	var solutions []Solution
	err = filepath.WalkDir(absBase, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if defaultExcludes[name] {
				return filepath.SkipDir
			}
			if matcher.match(absBase, path, name) {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.EqualFold(filepath.Ext(path), ".sln") {
			rel, _ := filepath.Rel(absBase, path)
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			solutions = append(solutions, Solution{
				Name:    name,
				AbsPath: path,
				RelPath: rel,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return solutions, nil
}

// excludeMatcher はユーザ定義の除外パターンを事前解析して保持する。
type excludeMatcher struct {
	names    map[string]bool // 名前完全一致 ("packages", "**/packages" → "packages")
	prefixes []string        // プレフィックスマッチ ("legacy/**" → "legacy")
	globs    []string        // filepath.Match 互換パターン
}

func newExcludeMatcher(patterns []string) excludeMatcher {
	m := excludeMatcher{names: make(map[string]bool)}
	for _, p := range patterns {
		p = filepath.ToSlash(p)
		switch {
		case strings.HasPrefix(p, "**/"):
			// "**/packages" → ディレクトリ名 "packages" の完全一致
			m.names[strings.TrimPrefix(p, "**/")] = true
		case strings.HasSuffix(p, "/**"):
			// "legacy/**" → "legacy" プレフィックスにマッチ
			m.prefixes = append(m.prefixes, strings.TrimSuffix(p, "/**"))
		case !strings.ContainsAny(p, "*?["):
			// ワイルドカードなし → ディレクトリ名の完全一致として扱う
			m.names[p] = true
		default:
			m.globs = append(m.globs, p)
		}
	}
	return m
}

// match はディレクトリがいずれかの除外パターンに一致するかを判定する。
// absBase は探索ルートの絶対パス、absPath はチェック対象の絶対パス、
// name はディレクトリのベース名。
func (em excludeMatcher) match(absBase, absPath, name string) bool {
	if em.names[name] {
		return true
	}

	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}
	relSlash := filepath.ToSlash(rel)

	for _, prefix := range em.prefixes {
		if relSlash == prefix || strings.HasPrefix(relSlash, prefix+"/") {
			return true
		}
	}

	for _, glob := range em.globs {
		if matched, _ := filepath.Match(glob, name); matched {
			return true
		}
		if matched, _ := filepath.Match(glob, relSlash); matched {
			return true
		}
	}

	return false
}

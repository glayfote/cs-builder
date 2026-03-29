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

// Scan は baseDir 配下を再帰的に走査し、見つかった .sln ファイルの一覧を返す。
//
// 走査時に以下のディレクトリは探索対象から除外される:
//   - bin, obj : .NET ビルド成果物ディレクトリ
//   - .git     : Git リポジトリメタデータ
//   - node_modules : Node.js 依存パッケージ
//
// 拡張子の比較は大文字小文字を無視する (例: .SLN も検出対象)。
// 個別ファイルの読み取りエラーはスキップし、走査を継続する。
func Scan(baseDir string) ([]Solution, error) {
	// 相対パス計算の基準として絶対パスに正規化する
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}

	var solutions []Solution
	err = filepath.WalkDir(absBase, func(path string, d fs.DirEntry, err error) error {
		// 個別エントリのエラー (権限不足等) はスキップして走査を継続
		if err != nil {
			return nil
		}

		// 探索不要なディレクトリをスキップしてパフォーマンスを確保
		if d.IsDir() {
			name := d.Name()
			if name == "bin" || name == "obj" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// 拡張子が .sln のファイルを収集する (大文字小文字を無視)
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

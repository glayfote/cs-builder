// Package sln は Visual Studio ソリューション（.sln）テキストからプロジェクトエントリを取り出す。
// 完全な MSBuild パーサではなく、Project(...) 行の正規表現抽出に依存する点に注意する。
package sln

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// projectLineRe は Solution 行の第 2 引数（.csproj への相対パス）を取り出す。
// 例: Project("{...}") = "name", "sub\a.csproj", "{guid}"
var projectLineRe = regexp.MustCompile(`(?i)^Project\s*\([^)]*\)\s*=\s*"[^"]*"\s*,\s*"([^"]+)"\s*,`)

// CsprojPathsFromSolution は .sln に含まれるプロジェクトの .csproj への絶対パスを返す。
//
// ソリューションファイルと同じディレクトリを基準に、第 2 引数の相対パスを解決する。
// 拡張子が .csproj（大文字小文字無視）の行のみ対象。重複パスは除き、結果は辞書順でソートする。
//
// ソリューションフォルダ行やネストされた構造のすべてのバリエーションはサポートしない。
func CsprojPathsFromSolution(slnPath string) ([]string, error) {
	slnPath = filepath.Clean(slnPath)
	data, err := os.ReadFile(slnPath)
	if err != nil {
		return nil, fmt.Errorf("read solution %q: %w", slnPath, err)
	}
	baseDir := filepath.Dir(slnPath)
	lines := strings.Split(string(data), "\n")
	seen := make(map[string]struct{})
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		m := projectLineRe.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		rel := filepath.FromSlash(strings.TrimSpace(m[1]))
		// ソリューションに .csproj 以外（共有プロジェクト等）が混ざる場合は無視する。
		if rel == "" || !strings.EqualFold(filepath.Ext(rel), ".csproj") {
			continue
		}
		abs := filepath.Clean(filepath.Join(baseDir, rel))
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no .csproj entries in solution %q", slnPath)
	}
	sort.Strings(out)
	return out, nil
}

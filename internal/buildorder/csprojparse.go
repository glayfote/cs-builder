package buildorder

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	// projectRefRe は <ProjectReference Include="相対または絶対パス" /> を拾う。
	projectRefRe = regexp.MustCompile(`(?i)<ProjectReference\s+Include\s*=\s*"([^"]+)"`)
	// assemblyRe は <AssemblyName>...</AssemblyName> を拾う（未設定時はファイル名をフォールバックする）。
	assemblyRe = regexp.MustCompile(`(?i)<AssemblyName>\s*([^<]+?)\s*</AssemblyName>`)
	// hintPathRe は <HintPath>...</HintPath> をすべて列挙する（Reference ブロック内の DLL パス）。
	hintPathRe = regexp.MustCompile(`(?i)<HintPath>\s*([^<]+?)\s*</HintPath>`)
)

// parseCsprojMeta は 1 つの .csproj を読み、次の 3 つを返す。
//
//   - assemblyName: 生成アセンブリ名（ビルド成果物のベース名と一致することが多い）
//   - projDeps: ProjectReference のターゲット .csproj の絶対パス一覧
//   - dllHintBases: HintPath で参照される .dll のファイル名から拡張子を除いた文字列一覧
//     （scan_roots 内の AssemblyName インデックスと突き合わせる）
//
// いずれも重複は除き、パスはクリーン済み・ソート済みで返す。
func parseCsprojMeta(csprojAbs string) (assemblyName string, projDeps []string, dllHintBases []string, err error) {
	csprojAbs = filepath.Clean(csprojAbs)
	data, err := os.ReadFile(csprojAbs)
	if err != nil {
		return "", nil, nil, fmt.Errorf("read %q: %w", csprojAbs, err)
	}
	text := string(data)
	baseDir := filepath.Dir(csprojAbs)

	if m := assemblyRe.FindStringSubmatch(text); len(m) >= 2 {
		assemblyName = strings.TrimSpace(m[1])
	}
	if assemblyName == "" {
		// SDK 既定: ファイル名（拡張子除く）がアセンブリ名になることが多い。
		assemblyName = strings.TrimSuffix(filepath.Base(csprojAbs), filepath.Ext(csprojAbs))
	}

	for _, m := range projectRefRe.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 {
			continue
		}
		raw := strings.TrimSpace(m[1])
		if raw == "" {
			continue
		}
		// Include はほぼ常に .csproj ディレクトリからの相対パス。
		depAbs := filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(raw)))
		projDeps = append(projDeps, depAbs)
	}

	for _, m := range hintPathRe.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 {
			continue
		}
		hp := strings.TrimSpace(m[1])
		if hp == "" {
			continue
		}
		hp = filepath.FromSlash(hp)
		if !strings.EqualFold(filepath.Ext(hp), ".dll") {
			continue
		}
		resolved := filepath.Clean(filepath.Join(baseDir, hp))
		base := strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))
		if base != "" {
			dllHintBases = append(dllHintBases, base)
		}
	}

	return assemblyName, uniqSorted(projDeps), uniqSorted(dllHintBases), nil
}

// uniqSorted はスライスを重複除去し、辞書順に整列する（キーは filepath.Clean 後の文字列）。
func uniqSorted(in []string) []string {
	seen := make(map[string]struct{})
	for _, s := range in {
		s = filepath.Clean(s)
		if s == "" {
			continue
		}
		seen[s] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

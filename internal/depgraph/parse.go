// Package depgraph は .csproj の HintPath 参照を解析し、
// ソリューション間の依存関係を DAG (有向非巡回グラフ) として構築する。
//
// ビルド順序の自動決定に使用され、トポロジカルソートにより
// 依存先が必ず先にビルドされる順序を保証する。
package depgraph

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// csprojDeps は .csproj ファイルから AssemblyName と HintPath 参照を
// 抽出するための XML 構造体。
type csprojDeps struct {
	PropertyGroups []depPropertyGroup `xml:"PropertyGroup"`
	ItemGroups     []depItemGroup     `xml:"ItemGroup"`
}

type depPropertyGroup struct {
	AssemblyName string `xml:"AssemblyName"`
}

type depItemGroup struct {
	References []depReference `xml:"Reference"`
}

type depReference struct {
	Include  string `xml:"Include,attr"`
	HintPath string `xml:"HintPath"`
}

// projectDeps は .csproj から抽出した依存解決に必要な情報を保持する。
type projectDeps struct {
	AssemblyName string   // このプロジェクトが生成するアセンブリ名
	Dependencies []string // HintPath から抽出した依存先アセンブリ名の一覧
}

// parseCsprojDeps は .csproj ファイルから AssemblyName と HintPath 依存先を抽出する。
func parseCsprojDeps(path string) (projectDeps, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return projectDeps{}, fmt.Errorf(".csproj の読み込みに失敗: %w", err)
	}

	var proj csprojDeps
	if err := xml.Unmarshal(data, &proj); err != nil {
		return projectDeps{}, fmt.Errorf(".csproj のパースに失敗: %w", err)
	}

	var assemblyName string
	for _, pg := range proj.PropertyGroups {
		if pg.AssemblyName != "" {
			assemblyName = pg.AssemblyName
		}
	}
	if assemblyName == "" {
		base := filepath.Base(path)
		assemblyName = strings.TrimSuffix(base, filepath.Ext(base))
	}

	var deps []string
	for _, ig := range proj.ItemGroups {
		for _, ref := range ig.References {
			name := extractAssemblyFromHintPath(ref.HintPath)
			if name == "" {
				name = ref.Include
			}
			if name != "" {
				deps = append(deps, name)
			}
		}
	}

	return projectDeps{
		AssemblyName: assemblyName,
		Dependencies: deps,
	}, nil
}

// extractAssemblyFromHintPath は HintPath からアセンブリ名を抽出する。
// 例: "..\..\5_dll\common\if\if_a\Pfm.Common.IfA.dll" → "Pfm.Common.IfA"
func extractAssemblyFromHintPath(hintPath string) string {
	if hintPath == "" {
		return ""
	}
	base := filepath.Base(filepath.FromSlash(hintPath))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Package artifact はビルド成果物 (DLL/PDB) の管理を担当する。
//
// ビルド完了後に成果物を共有 DLL ディレクトリにコピーすることで、
// 他のプロジェクトが HintPath 経由で参照できるようにする。
package artifact

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// csproj は .csproj ファイルから必要な情報だけを抽出するための XML 構造体。
type csproj struct {
	PropertyGroups []propertyGroup `xml:"PropertyGroup"`
}

type propertyGroup struct {
	AssemblyName    string `xml:"AssemblyName"`
	TargetFramework string `xml:"TargetFramework"`
}

// projectInfo は .csproj から抽出したビルド成果物の特定に必要な情報を保持する。
type projectInfo struct {
	Dir             string // .csproj が存在するディレクトリの絶対パス
	AssemblyName    string // 出力アセンブリ名 (省略時は .csproj のファイル名)
	TargetFramework string // ターゲットフレームワーク (例: "net8.0"、空なら非 SDK スタイル)
}

// CopyArtifact はビルド済みの成果物を共有 DLL ディレクトリにコピーする。
//
// slnPath から同ディレクトリの .csproj を探し、AssemblyName と TargetFramework を読み取って
// ビルド出力パスを組み立て、DLL と PDB (存在する場合) を sharedDllDir にコピーする。
//
// sharedDllDir が存在しない場合は自動作成する。
// PDB が存在しない場合は DLL のみコピーしエラーにしない。
func CopyArtifact(slnPath string, configuration string, sharedDllDir string) error {
	info, err := findAndParseProject(slnPath)
	if err != nil {
		return err
	}

	outputDir := buildOutputDir(info, configuration)

	if err := os.MkdirAll(sharedDllDir, 0o755); err != nil {
		return fmt.Errorf("共有 DLL ディレクトリの作成に失敗: %w", err)
	}

	dllName := info.AssemblyName + ".dll"
	src := filepath.Join(outputDir, dllName)
	dst := filepath.Join(sharedDllDir, dllName)
	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("%s のコピーに失敗: %w", dllName, err)
	}

	pdbName := info.AssemblyName + ".pdb"
	pdbSrc := filepath.Join(outputDir, pdbName)
	pdbDst := filepath.Join(sharedDllDir, pdbName)
	if _, err := os.Stat(pdbSrc); err == nil {
		if err := copyFile(pdbSrc, pdbDst); err != nil {
			return fmt.Errorf("%s のコピーに失敗: %w", pdbName, err)
		}
	}

	return nil
}

// findAndParseProject は .sln と同ディレクトリにある .csproj を探してパースする。
func findAndParseProject(slnPath string) (projectInfo, error) {
	dir := filepath.Dir(slnPath)
	matches, err := filepath.Glob(filepath.Join(dir, "*.csproj"))
	if err != nil {
		return projectInfo{}, fmt.Errorf(".csproj の検索に失敗: %w", err)
	}
	if len(matches) == 0 {
		return projectInfo{}, fmt.Errorf(".csproj が見つかりません: %s", dir)
	}

	csprojPath := matches[0]
	return parseCsproj(csprojPath)
}

// parseCsproj は .csproj ファイルから AssemblyName と TargetFramework を抽出する。
func parseCsproj(path string) (projectInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return projectInfo{}, fmt.Errorf(".csproj の読み込みに失敗: %w", err)
	}

	var proj csproj
	if err := xml.Unmarshal(data, &proj); err != nil {
		return projectInfo{}, fmt.Errorf(".csproj のパースに失敗: %w", err)
	}

	info := projectInfo{
		Dir: filepath.Dir(path),
	}

	for _, pg := range proj.PropertyGroups {
		if pg.AssemblyName != "" {
			info.AssemblyName = pg.AssemblyName
		}
		if pg.TargetFramework != "" {
			info.TargetFramework = pg.TargetFramework
		}
	}

	// AssemblyName 省略時は .csproj のファイル名をフォールバック (MSBuild の標準動作)
	if info.AssemblyName == "" {
		base := filepath.Base(path)
		info.AssemblyName = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return info, nil
}

// buildOutputDir はビルド成果物の出力ディレクトリパスを組み立てる。
//
// SDK スタイル (TargetFramework あり):
//
//	<project_dir>/bin/<Configuration>/<TargetFramework>/
//
// 非 SDK スタイル (TargetFramework なし):
//
//	<project_dir>/bin/<Configuration>/
func buildOutputDir(info projectInfo, configuration string) string {
	if configuration == "" {
		configuration = "Debug"
	}
	if info.TargetFramework != "" {
		return filepath.Join(info.Dir, "bin", configuration, info.TargetFramework)
	}
	return filepath.Join(info.Dir, "bin", configuration)
}

// copyFile は src から dst にファイルをコピーする。
// dst が既に存在する場合は上書きする。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

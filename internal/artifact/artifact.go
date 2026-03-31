// Package artifact はビルド成果物 (DLL/PDB) の管理を担当する。
//
// ビルド完了後に成果物を共有 DLL ディレクトリにコピーすることで、
// 他のプロジェクトが HintPath 経由で参照できるようにする。
package artifact

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

// SlnProjectRe は .sln ファイルの Project 行から相対パスを抽出する。
// 形式: Project("{TypeGUID}") = "Name", "RelPath", "{GUID}"
var SlnProjectRe = regexp.MustCompile(
	`^Project\("\{[^}]+\}"\)\s*=\s*"[^"]+",\s*"([^"]+)",\s*"\{[^}]+\}"`,
)

// CopyArtifact はビルド済みの成果物を共有 DLL ディレクトリにコピーする。
//
// slnPath をパースして参照先の .csproj を特定し、AssemblyName と TargetFramework を読み取って
// ビルド出力パスを組み立て、DLL / PDB / deps.json を sharedDllDir にコピーする。
//
// baseDir (scan root) からの相対パスでディレクトリ構造を保持する。
// 例: baseDir="pfm", slnPath="pfm/3_common/if/if_a/if_a.sln"
//   → コピー先: sharedDllDir/3_common/if/if_a/
//
// sharedDllDir が存在しない場合は自動作成する。
// PDB / deps.json が存在しない場合はスキップしエラーにしない。
func CopyArtifact(slnPath, configuration, sharedDllDir, baseDir string) error {
	info, err := findAndParseProject(slnPath)
	if err != nil {
		return err
	}

	outputDir := buildOutputDir(info, configuration)

	relDir, err := filepath.Rel(baseDir, filepath.Dir(slnPath))
	if err != nil {
		return fmt.Errorf("相対パスの算出に失敗: %w", err)
	}
	dstDir := filepath.Join(sharedDllDir, relDir)

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("コピー先ディレクトリの作成に失敗: %w", err)
	}

	dllName := info.AssemblyName + ".dll"
	if err := copyFile(filepath.Join(outputDir, dllName), filepath.Join(dstDir, dllName)); err != nil {
		return fmt.Errorf("%s のコピーに失敗: %w", dllName, err)
	}

	for _, name := range []string{
		info.AssemblyName + ".pdb",
		info.AssemblyName + ".deps.json",
	} {
		src := filepath.Join(outputDir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(dstDir, name)); err != nil {
			return fmt.Errorf("%s のコピーに失敗: %w", name, err)
		}
	}

	return nil
}

// findAndParseProject は .sln をパースして参照先の .csproj を特定し、パースする。
func findAndParseProject(slnPath string) (projectInfo, error) {
	csprojRel, err := ExtractCsprojPath(slnPath)
	if err != nil {
		return projectInfo{}, err
	}

	csprojPath := filepath.Join(filepath.Dir(slnPath), csprojRel)
	return parseCsproj(csprojPath)
}

// ExtractCsprojPath は .sln ファイルから最初の .csproj 参照の相対パスを返す。
func ExtractCsprojPath(slnPath string) (string, error) {
	f, err := os.Open(slnPath)
	if err != nil {
		return "", fmt.Errorf(".sln の読み込みに失敗: %w", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := SlnProjectRe.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		relPath := m[1]
		if strings.EqualFold(filepath.Ext(relPath), ".csproj") {
			return filepath.FromSlash(relPath), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf(".sln の読み取りエラー: %w", err)
	}
	return "", fmt.Errorf(".sln に .csproj の参照が見つかりません: %s", slnPath)
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

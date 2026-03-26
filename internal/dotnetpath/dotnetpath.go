// Package dotnetpath は dotnet CLI の実行ファイルパスを決定する。
package dotnetpath

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// EnvOverride は PATH より優先して参照する環境変数名（フルパス、または dotnet を含むディレクトリ）。
const EnvOverride = "CS_BUILDER_DOTNET"

// Resolve は dotnet 実行ファイルの絶対パスを返す。
//
// 探索順:
//  1. 環境変数 CS_BUILDER_DOTNET（ファイルパス、または dotnet が入っているディレクトリ）
//  2. exec.LookPath("dotnet")
//  3. 環境変数 DOTNET_ROOT（SDK が設定するインストールルート）配下の dotnet[.exe]
//  4. Windows: %ProgramFiles%\dotnet\dotnet.exe および %ProgramW6432%\dotnet\dotnet.exe
func Resolve() (string, error) {
	if raw := strings.TrimSpace(os.Getenv(EnvOverride)); raw != "" {
		return normalizeOverride(raw)
	}
	if p, err := exec.LookPath("dotnet"); err == nil {
		return filepath.Clean(p), nil
	}
	if root := strings.TrimSpace(os.Getenv("DOTNET_ROOT")); root != "" {
		if p, ok := dotnetUnderRoot(root); ok {
			return p, nil
		}
	}
	if runtime.GOOS == "windows" {
		for _, pf := range []string{os.Getenv("ProgramW6432"), os.Getenv("ProgramFiles")} {
			if pf == "" {
				continue
			}
			p := filepath.Join(pf, "dotnet", "dotnet.exe")
			if isRegularFile(p) {
				return filepath.Clean(p), nil
			}
		}
	}
	return "", fmt.Errorf(
		"dotnet が見つかりません。.NET SDK をインストールするか、PATH に dotnet を通してください。"+
			" 環境変数 %s に dotnet のフルパス（またはインストールディレクトリ）を指定することもできます。",
		EnvOverride,
	)
}

func normalizeOverride(raw string) (string, error) {
	st, err := os.Stat(raw)
	if err != nil {
		return "", fmt.Errorf("%s %q: %w", EnvOverride, raw, err)
	}
	if st.IsDir() {
		name := "dotnet"
		if runtime.GOOS == "windows" {
			name = "dotnet.exe"
		}
		p := filepath.Join(raw, name)
		if !isRegularFile(p) {
			return "", fmt.Errorf("%s ディレクトリ %q に %s がありません", EnvOverride, raw, name)
		}
		return filepath.Clean(p), nil
	}
	return filepath.Clean(raw), nil
}

func dotnetUnderRoot(root string) (string, bool) {
	name := "dotnet"
	if runtime.GOOS == "windows" {
		name = "dotnet.exe"
	}
	p := filepath.Join(root, name)
	if isRegularFile(p) {
		return filepath.Clean(p), true
	}
	return "", false
}

func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

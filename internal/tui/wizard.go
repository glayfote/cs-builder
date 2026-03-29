// Package tui は Bubble Tea による対話ウィザード（構成・テナント・.sln 選択と dotnet ビルド）を提供する。
package tui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"builder/cs-builder/internal/config"
	"builder/cs-builder/internal/scan"
)

// dotnet build -c に渡す構成名。
const (
	ConfigDebug   = "Debug"
	ConfigRelease = "Release"
)

// テナント選択の値（スキャン結果の Solution.Tenant フィルタに対応。TenantAll は絞り込みなし）。
const (
	TenantAll = "all"
	Tenant1   = "tenant1"
	Tenant2   = "tenant2"
	Tenant3   = "tenant3"
	Tenant4   = "tenant4"
)

// ErrUserAbort はユーザーが q / Esc 等でウィザードを中断したときに RunWizard が返す。
// 設定ファイル不正やスキャンエラーとは区別する。
var ErrUserAbort = errors.New("user aborted wizard")

var tenantChoices = []string{Tenant1, Tenant2, Tenant3, Tenant4, TenantAll}

// BuildFailure は 1 回の dotnet build（.csproj 単位）が失敗したときのパスとエラー内容。
type BuildFailure struct {
	Path string
	Err  error
}

// WizardResult はウィザードが正常終了しサマリ画面まで到達したときのスナップショット。
// 呼び出し側は BuildFailures の有無でプロセス終了コードを決められる。
type WizardResult struct {
	Configuration string
	Tenant        string
	// SolutionPaths はユーザーがチェックした .sln の絶対パス（UI 上の選択結果）。
	SolutionPaths []string
	// OrderedProjectPaths は buildorder.Resolve が算出した .csproj のビルド順。
	OrderedProjectPaths []string
	BuildFailures       []BuildFailure
}

// RunWizard は設定の読込・パス検証・.sln スキャンの後、TTY 上でウィザードを起動する。
//
// configPath は Cobra の --config に相当するパス（空なら環境変数・カレントの既定 YAML）。
// ユーザー中断時は (WizardResult{}, ErrUserAbort)、その他のエラーはゼロ値結果とともに返す。
func RunWizard(configPath string) (WizardResult, error) {
	cfg, _, err := config.Load(configPath)
	if err != nil {
		return WizardResult{}, err
	}
	if err := cfg.ValidatePaths(); err != nil {
		return WizardResult{}, err
	}
	all, err := scan.FindSolutions(cfg)
	if err != nil {
		return WizardResult{}, err
	}

	m := newWizardModel(all, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return WizardResult{}, err
	}
	wm, ok := final.(*wizardModel)
	if !ok {
		return WizardResult{}, fmt.Errorf("tui: unexpected model type %T", final)
	}
	if wm.aborted {
		return WizardResult{}, ErrUserAbort
	}
	return wm.finalResult, nil
}

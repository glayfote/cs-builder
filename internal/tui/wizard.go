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

// テナント選択の値（スキャン結果の Solution.Tenant フィルタに対応。all は絞り込みなし）。
const (
	Tenant1   = "tenant1"
	Tenant2   = "tenant2"
	Tenant3   = "tenant3"
	Tenant4   = "tenant4"
	TenantAll = "all"
)

// ErrUserAbort はユーザーが中断したときに返す（設定・スキャンエラーではない）。
var ErrUserAbort = errors.New("user aborted wizard")

var tenantChoices = []string{Tenant1, Tenant2, Tenant3, Tenant4, TenantAll}

var packageModeChoices = []string{"単一パッケージ", "複数パッケージ"}

var pickKindChoices = []string{"パッケージを個別に選ぶ", "フォルダを選び配下をすべて"}

// BuildFailure は 1 ソリューションのビルド失敗。
type BuildFailure struct {
	Path string
	Err  error
}

// WizardResult はウィザード完了後の結果（Summary で終了したとき）。
type WizardResult struct {
	Configuration  string
	Tenant         string
	SinglePackage  bool
	PickIndividual bool
	SolutionPaths  []string
	BuildFailures  []BuildFailure
}

// RunWizard は設定読込・スキャン後、TTY ウィザードを実行する。
// configPath は Cobra の --config（空なら環境変数・cwd の既定）。
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

	m := newWizardModel(all)
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

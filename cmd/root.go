// Package cmd は Cobra ベースの CLI ルートコマンドを定義する。
//
// ルートコマンドは以下のフラグを受け取り、Bubble Tea TUI を起動する:
//   - --path (-p)     : .sln ファイルを探索するベースディレクトリ（省略時はカレントディレクトリ）
//   - --config (-c)   : MSBuild に渡すビルド構成 (Debug / Release)
//   - --build-cmd     : 使用するビルドコマンド ("dotnet" または "msbuild")
//
// TUI が正常終了した場合は nil を返し、内部エラーが発生した場合は
// Model.Err に格納されたエラーを呼び出し元に伝播する。
package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"builder/cs-builder/internal/tui"
)

// CLI フラグのバインド先。init() で Cobra フラグに紐付ける。
var (
	flagPath     string // --path: スキャン対象ディレクトリ
	flagConfig   string // --config: ビルド構成 (Debug/Release)
	flagBuildCmd string // --build-cmd: ビルドコマンド (dotnet/msbuild)
)

// rootCmd はアプリケーションのルートコマンド。
// サブコマンドを持たず、引数なしで実行すると TUI が起動する。
var rootCmd = &cobra.Command{
	Use:   "cs-builder",
	Short: "C# ソリューションを対話的にビルドする TUI ツール",
	Long: `cs-builder はプロジェクト内の .sln ファイルを再帰探索し、
Bubble Tea ベースの対話 UI でソリューションを選択して
MSBuild (dotnet build / msbuild) でビルドします。

ビルド進捗はリアルタイムに表示され、完了後に各ソリューションの
成功/失敗サマリを確認できます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// --path が未指定の場合はカレントディレクトリをデフォルトとする
		if flagPath == "" {
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("カレントディレクトリの取得に失敗: %w", err)
			}
			flagPath = wd
		}

		// Bubble Tea のモデルを生成し、AltScreen モードで TUI を起動する。
		// AltScreen を使うことで、終了時に元のターミナル表示が復元される。
		m := tui.NewModel(flagPath, flagBuildCmd, flagConfig)
		p := tea.NewProgram(m, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("TUI の実行に失敗: %w", err)
		}

		// TUI 内部で発生したエラー（スキャン失敗等）を呼び出し元に伝播する
		if fm, ok := finalModel.(tui.Model); ok && fm.Err != nil {
			return fm.Err
		}
		return nil
	},
}

// init は Cobra のフラグを rootCmd に登録する。
// Cobra の初期化タイミングで自動的に呼ばれる。
func init() {
	rootCmd.Flags().StringVarP(&flagPath, "path", "p", "", "スキャン対象ディレクトリ (デフォルト: カレントディレクトリ)")
	rootCmd.Flags().StringVarP(&flagConfig, "config", "c", "Debug", "ビルド構成 (Debug / Release)")
	rootCmd.Flags().StringVar(&flagBuildCmd, "build-cmd", "dotnet", "ビルドコマンド (dotnet / msbuild)")
}

// Execute は Cobra のルートコマンドを実行する。
// main パッケージから呼び出されるエントリポイント。
// フラグ解析 → RunE コールバック → TUI 起動 の順に処理が進む。
func Execute() error {
	return rootCmd.Execute()
}

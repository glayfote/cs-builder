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
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"builder/cs-builder/internal/builder"
	"builder/cs-builder/internal/config"
	"builder/cs-builder/internal/logging"
	"builder/cs-builder/internal/tui"
)

// CLI フラグのバインド先。init() で Cobra フラグに紐付ける。
var (
	flagPath     string // --path: スキャン対象ディレクトリ
	flagConfig   string // --config: ビルド構成 (Debug/Release)
	flagBuildCmd string // --build-cmd: ビルドコマンド (dotnet/msbuild)
	flagParallel int    // --parallel: 同一レベル内の最大並列ビルド数
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

		// .cs-builder.toml を探索して読み込む (見つからなければゼロ値)
		cfg, err := config.Load(flagPath)
		if err != nil {
			return fmt.Errorf("設定ファイルの読み込みに失敗: %w", err)
		}

		// ログの初期化 (TOML の [log] セクション設定を使用)
		logFile, err := logging.Setup(cfg.Log.Dir, cfg.Log.Level)
		if err != nil {
			return fmt.Errorf("ログの初期化に失敗: %w", err)
		}
		defer logFile.Close()

		// CLI フラグ > TOML > デフォルト の優先順位でマージする。
		// cmd.Flags().Changed() で明示的に指定されたフラグを判定する。
		projectRoot := flagPath
		if !cmd.Flags().Changed("path") && cfg.Scan.ProjectRoot != "" {
			projectRoot = cfg.Scan.ProjectRoot
		}

		// scan.roots から scanner 用のパススライスと dllDirMap を組み立てる
		var scanRootPaths []string
		dllDirMap := make(map[string]string)
		for _, r := range cfg.Scan.Roots {
			scanRootPaths = append(scanRootPaths, r.Path)
			if r.SharedDllDir != "" {
				dir := r.SharedDllDir
				if !filepath.IsAbs(dir) {
					dir = filepath.Join(projectRoot, dir)
				}
				dllDirMap[r.Path] = dir
			}
		}

		buildCmd := flagBuildCmd
		if !cmd.Flags().Changed("build-cmd") && cfg.Defaults.BuildCmd != "" {
			buildCmd = cfg.Defaults.BuildCmd
		}

		buildConfig := flagConfig
		if !cmd.Flags().Changed("config") && cfg.Defaults.Config != "" {
			buildConfig = cfg.Defaults.Config
		}

		opts := builder.BuildOption{
			Command:       buildCmd,
			Configuration: buildConfig,
			DotnetPath:    cfg.Commands.Dotnet,
			MSBuildPath:   cfg.Commands.MSBuild,
		}

		maxParallel := flagParallel
		if !cmd.Flags().Changed("parallel") && cfg.Defaults.MaxParallel != 0 {
			maxParallel = cfg.Defaults.MaxParallel
		}

		slog.Info("cs-builder started",
			"projectRoot", projectRoot,
			"buildCmd", buildCmd,
			"config", buildConfig,
			"maxParallel", maxParallel,
			"scanRoots", scanRootPaths,
		)

		m := tui.NewModel(projectRoot, scanRootPaths, opts, cfg.Scan.Exclude, dllDirMap, maxParallel)
		p := tea.NewProgram(m, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			slog.Error("TUI execution failed", "error", err)
			return fmt.Errorf("TUI の実行に失敗: %w", err)
		}

		// TUI 内部で発生したエラー（スキャン失敗等）を呼び出し元に伝播する
		if fm, ok := finalModel.(tui.Model); ok && fm.Err != nil {
			slog.Error("TUI exited with error", "error", fm.Err)
			return fm.Err
		}
		slog.Info("cs-builder finished")
		return nil
	},
}

// init は Cobra のフラグを rootCmd に登録する。
// Cobra の初期化タイミングで自動的に呼ばれる。
func init() {
	rootCmd.Flags().StringVarP(&flagPath, "path", "p", "", "スキャン対象ディレクトリ (デフォルト: カレントディレクトリ)")
	rootCmd.Flags().StringVarP(&flagConfig, "config", "c", "Debug", "ビルド構成 (Debug / Release)")
	rootCmd.Flags().StringVar(&flagBuildCmd, "build-cmd", "dotnet", "ビルドコマンド (dotnet / msbuild)")
	rootCmd.Flags().IntVar(&flagParallel, "parallel", 0, "同一レベル内の最大並列ビルド数 (0=無制限)")
}

// Execute は Cobra のルートコマンドを実行する。
// main パッケージから呼び出されるエントリポイント。
// フラグ解析 → RunE コールバック → TUI 起動 の順に処理が進む。
func Execute() error {
	return rootCmd.Execute()
}

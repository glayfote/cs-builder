package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"builder/cs-builder/internal/config"
	"builder/cs-builder/internal/scan"
)

// scanVerbose が true のとき、使用した設定ファイルパスと各 .sln のメタ情報を stderr に出す。
var scanVerbose bool

// scanCmd は設定に基づきモノレポ内の .sln を探索し、結果を標準出力に列挙する。
// 設定の読み込みは config.Load（--config / 環境変数 / cwd の cs-builder.yaml）。
// 各行 1 パスの絶対パスを stdout に出すため、シェルスクリプトからパイプしやすい。
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "List discovered .sln files from config",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. YAML を読み、ApplyDefaults・Validate 済みの Config を得る。
		cfg, usedPath, err := config.Load(configPath)
		if err != nil {
			return err
		}
		// 2. project_root と各 scan_roots が実在するか確認（探索前に失敗させる）。
		if err := cfg.ValidatePaths(); err != nil {
			return err
		}
		// 3. scan_roots 配下を走査し、重複を除いた Solution 一覧を取得。
		solutions, err := scan.FindSolutions(cfg)
		if err != nil {
			return err
		}
		if scanVerbose {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "config: %s\n", usedPath)
		}
		// 常に各 .sln の絶対パスを 1 行ずつ stdout へ。verbose 時は stderr にスキャンルート等の補足も出す。
		out := cmd.OutOrStdout()
		for _, s := range solutions {
			if scanVerbose {
				if s.Tenant != "" {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s  (%s / %s / %s)\n", s.Path, s.ScanRoot, s.PackageDir, s.Tenant)
				} else {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s  (%s / %s)\n", s.Path, s.ScanRoot, s.PackageDir)
				}
			}
			_, _ = fmt.Fprintln(out, s.Path)
		}
		return nil
	},
}

func init() {
	scanCmd.Flags().BoolVar(&scanVerbose, "verbose", false, "print config path and labels to stderr")
	rootCmd.AddCommand(scanCmd)
}

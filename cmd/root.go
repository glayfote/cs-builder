package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"builder/cs-builder/internal/tui"
)

var configPath string

var rootCmd = &cobra.Command{
	Use:   "cs-builder",
	Short: "C# monorepo build helper",
	Long: `C# モノレポ向けビルド補助ツール。

サブコマンドなしで起動し、標準入力がターミナル（TTY）のときは対話ウィザードで
構成・テナント・パッケージ対象を選び、dotnet build を実行します。
非対話環境では scan などのサブコマンドを指定してください。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			_, _ = fmt.Fprintln(os.Stderr, "cs-builder: 標準入力がターミナルではありません。サブコマンドを指定してください（例: cs-builder scan）。")
			return errors.New("non-interactive stdin")
		}
		res, err := tui.RunWizard(configPath)
		if err != nil {
			if errors.Is(err, tui.ErrUserAbort) {
				_, _ = fmt.Fprintln(os.Stderr, "中断しました。")
			}
			return err
		}
		if len(res.BuildFailures) > 0 {
			return fmt.Errorf("%d 件のビルドが失敗しました", len(res.BuildFailures))
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "path to cs-builder.yaml (overrides CS_BUILDER_CONFIG)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// buildCmd は非対話の一括ビルド用に予約されたサブコマンド（未実装）。
// 対話ビルドはルートコマンドの RunE（TTY 時ウィザード）で行う。
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build solutions (not implemented yet)",
	Run: func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "build: not implemented")
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

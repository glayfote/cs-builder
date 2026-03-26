// Package cmd は Cobra によるルートコマンドとサブコマンド（scan / build 等）を定義する。
// 引数なし実行時は TTY 判定のうえ Bubble Tea ウィザード（internal/tui）へ遷移する。
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"builder/cs-builder/internal/tui"
)

// configPath は全サブコマンド共通の --config フラグ値（空のときは環境変数・カレントの既定 YAML）。
var configPath string

var rootCmd = &cobra.Command{
	Use:   "cs-builder",
	Short: "C# monorepo build helper",
	Long: `C# モノレポ向けビルド補助ツール。

サブコマンドなしで起動し、標準入力がターミナル（TTY）のときは対話ウィザードで
構成・テナント・ビルド対象の .sln を選び、各 .csproj の依存（ProjectReference と
HintPath の .dll）から DAG を作り、依存のない順で dotnet build を実行します。
非対話環境では scan などのサブコマンドを指定してください。

dotnet が PATH に無い場合は、環境変数 CS_BUILDER_DOTNET に dotnet のフルパス（または SDK インストールディレクトリ）を指定できます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// CI やパイプでは stdin が TTY にならないため、誤ってウィザードを起動しない。
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
		// 1 件でも dotnet が非ゼロ終了ならプロセス全体を失敗扱いにする。
		if len(res.BuildFailures) > 0 {
			return fmt.Errorf("%d 件のビルドが失敗しました", len(res.BuildFailures))
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "path to cs-builder.yaml (overrides CS_BUILDER_CONFIG)")
}

// Execute はルート Cobra コマンドを実行する。main からのみ呼ばれる想定。
// 返り値の error が非 nil のとき main は終了コード 1 で終了する。
func Execute() error {
	return rootCmd.Execute()
}

// cs-builder は C# モノレポ向けの CLI エントリポイント。
// 実際のサブコマンドと対話フローは cmd パッケージに委譲する。
package main

import (
	"os"

	"builder/cs-builder/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// Cobra / ウィザードは既にメッセージを出している想定。終了コードのみ非ゼロにする。
		os.Exit(1)
	}
}

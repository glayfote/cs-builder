// cs-builder は C# モノレポ向けの対話型ビルド TUI ツールのエントリポイント。
//
// このパッケージはアプリケーションの起動のみを担当し、
// 実際の CLI 定義・TUI フローは cmd パッケージに委譲する。
// Cobra によるフラグ解析と Bubble Tea による対話 UI を組み合わせ、
// プロジェクト内の .sln ファイルを探索・選択・ビルドするワークフローを提供する。
//
// 使用例:
//
//	cs-builder --path ./src --config Release --build-cmd dotnet
//	cs-builder -p testdata/monorepo/pfm -c Debug
package main

import (
	"os"

	"builder/cs-builder/cmd"
)

// main はアプリケーションのエントリポイント。
// cmd.Execute() が返すエラーは Cobra が既にユーザへメッセージを出力済みのため、
// ここでは終了コードを非ゼロに設定するだけで追加の出力は行わない。
func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Package tui は Bubble Tea ベースの対話型ビルド TUI を提供する。
//
// このファイルでは TUI 全体で共有する lipgloss スタイル定数を定義する。
// ANSI 256 色パレットのインデックスを使用しており、
// ほぼすべてのモダンターミナルで正しく表示される。
//
// 色番号の対応:
//
//	7  = 白 (通常テキスト)
//	8  = 明るいグレー (補助テキスト、ヘルプ)
//	9  = 赤 (失敗・エラー)
//	10 = 緑 (成功・選択済み)
//	11 = 黄 (進捗表示)
//	12 = 青 (タイトル)
//	13 = マゼンタ (スピナー)
//	14 = シアン (カーソル)
package tui

import "github.com/charmbracelet/lipgloss"

var (
	// titleStyle はアプリケーションのタイトル行に使用するスタイル。
	// 太字の青色で、下に 1 行分のマージンを設ける。
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1)

	// subtitleStyle は補足情報 (件数表示等) に使用するグレーのスタイル。
	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			MarginBottom(1)

	// cursorStyle はリスト内の現在のカーソル位置を強調するスタイル。
	// シアン太字で視認性を高める。
	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Bold(true)

	// selectedStyle は選択済み項目のテキストに適用する緑色スタイル。
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	// unselectedStyle は未選択項目のテキストに適用する白色スタイル。
	unselectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	// successStyle はビルド成功の表示に使用する緑太字スタイル。
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)

	// failureStyle はビルド失敗の表示に使用する赤太字スタイル。
	failureStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)

	// spinnerStyle はスピナーアニメーション文字に適用するマゼンタスタイル。
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13"))

	// logStyle はビルドログの各行に適用するグレーのスタイル。
	// 本文より控えめな色にすることで、ステータス情報と視覚的に区別する。
	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	// progressStyle はビルド進捗テキスト (例: "進捗: 2 / 5") に使用する黄色太字スタイル。
	progressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	// helpStyle は画面下部のキーバインドヘルプに使用するグレーのスタイル。
	// 上に 1 行分のマージンを設けてコンテンツと分離する。
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			MarginTop(1)

	// headerBorder はセクション区切り (例: "ビルドログ") に使用するスタイル。
	// 下罫線付きで、セクション間の視覚的な境界を作る。
	headerBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("8")).
			MarginBottom(1)

	// filterPromptStyle はフィルタ入力行のプロンプト ("フィルタ:") に使用するスタイル。
	// シアン太字で、入力モード中であることを視覚的に示す。
	filterPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("14")).
				Bold(true)

	// filterTextStyle はフィルタ入力中のテキストに使用する黄色スタイル。
	filterTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	// scrollIndicatorStyle はスクロールバーのインジケータに使用するグレーのスタイル。
	scrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))

	// scrollThumbStyle はスクロールバーのつまみ (現在位置) に使用するスタイル。
	scrollThumbStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("12"))
)

package tui

import (
	"fmt"
	"strings"

	"builder/cs-builder/internal/scanner"
)

// selectModel はソリューション選択画面の状態を保持する。
// ユーザがカーソル移動・トグル操作でビルド対象を選択する。
//
// Bubble Tea のイミュータブルな更新パターンに従い、
// 各メソッドは値レシーバで新しい selectModel を返す。
type selectModel struct {
	solutions []scanner.Solution // 探索で見つかった全ソリューション
	cursor    int                // 現在のカーソル位置 (0-indexed)
	selected  map[int]bool       // 選択状態のマップ (キー: solutions のインデックス)
}

// newSelectModel は探索結果のソリューション一覧から selectModel を初期化する。
// 初期状態ではどのソリューションも未選択。
func newSelectModel(solutions []scanner.Solution) selectModel {
	return selectModel{
		solutions: solutions,
		selected:  make(map[int]bool),
	}
}

// cursorUp はカーソルを 1 つ上に移動する。
// 既にリストの先頭にいる場合は何もしない。
func (m selectModel) cursorUp() selectModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

// cursorDown はカーソルを 1 つ下に移動する。
// 既にリストの末尾にいる場合は何もしない。
func (m selectModel) cursorDown() selectModel {
	if m.cursor < len(m.solutions)-1 {
		m.cursor++
	}
	return m
}

// toggle は現在のカーソル位置のソリューションの選択状態を反転する。
// 選択済みなら解除し、未選択なら選択する。
func (m selectModel) toggle() selectModel {
	if m.selected[m.cursor] {
		delete(m.selected, m.cursor)
	} else {
		m.selected[m.cursor] = true
	}
	return m
}

// toggleAll は全ソリューションの選択状態を一括切り替えする。
// 全て選択済みの場合は全解除し、それ以外の場合は全選択する。
func (m selectModel) toggleAll() selectModel {
	if len(m.selected) == len(m.solutions) {
		m.selected = make(map[int]bool)
	} else {
		for i := range m.solutions {
			m.selected[i] = true
		}
	}
	return m
}

// selectedSolutions は現在選択されているソリューションのスライスを返す。
// solutions のインデックス順 (探索で見つかった順) で返される。
// ビルド開始時に呼ばれ、buildModel の初期化に使用される。
func (m selectModel) selectedSolutions() []scanner.Solution {
	var result []scanner.Solution
	for i, s := range m.solutions {
		if m.selected[i] {
			result = append(result, s)
		}
	}
	return result
}

// hasSelection は 1 つ以上のソリューションが選択されているかを返す。
// Enter キー押下時のバリデーションに使用される。
func (m selectModel) hasSelection() bool {
	return len(m.selected) > 0
}

// view はソリューション選択画面の表示文字列を生成する。
//
// 画面構成:
//
//	[タイトル]    "CS Builder"
//	[サブタイトル] "N 個のソリューションが見つかりました (M 個選択中)"
//	[リスト]      各ソリューションの相対パスを表示
//	              ▸ = カーソル位置、● = 選択済み、○ = 未選択
//	[ヘルプ]      キーバインドの説明
func (m selectModel) view() string {
	var b strings.Builder

	title := titleStyle.Render("CS Builder")
	b.WriteString(title + "\n")

	subtitle := subtitleStyle.Render(
		fmt.Sprintf("%d 個のソリューションが見つかりました (%d 個選択中)", len(m.solutions), len(m.selected)),
	)
	b.WriteString(subtitle + "\n")

	// 各ソリューションを 1 行ずつ描画する
	for i, s := range m.solutions {
		// カーソル位置の行には ▸ を表示し、それ以外は空白でインデントを揃える
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
		}

		// 選択状態に応じてアイコンとスタイルを切り替える
		check := "○"
		style := unselectedStyle
		if m.selected[i] {
			check = "●"
			style = selectedStyle
		}

		line := fmt.Sprintf("%s%s %s", cursor, check, style.Render(s.RelPath))
		b.WriteString(line + "\n")
	}

	help := helpStyle.Render("↑/↓: 移動  space: 選択  a: 全選択  enter: ビルド開始  q: 終了")
	b.WriteString("\n" + help)

	return b.String()
}

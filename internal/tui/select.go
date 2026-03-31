package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"builder/cs-builder/internal/scanner"
)

// selectModel はソリューション選択画面の状態を保持する。
// ユーザがカーソル移動・トグル操作でビルド対象を選択する。
//
// フィルタリング時は filtered スライスを介してソリューション一覧に間接アクセスし、
// カーソル位置やスクロールオフセットは filtered 内のインデックスで管理する。
//
// Bubble Tea のイミュータブルな更新パターンに従い、
// 各メソッドは値レシーバで新しい selectModel を返す。
type selectModel struct {
	solutions []scanner.Solution // 探索で見つかった全ソリューション
	cursor    int                // filtered 内のカーソル位置 (0-indexed)
	selected  map[int]bool       // 選択状態のマップ (キー: solutions の実インデックス)

	filter    string // 現在のフィルタ文字列
	filtering bool   // フィルタ入力モード中か
	filtered  []int  // フィルタに一致する solutions のインデックス一覧

	offset int // スクロール表示のオフセット (filtered 内の先頭表示位置)
}

// selectFixedLines はリスト領域以外の固定行数。
// タイトル (1) + MarginBottom (1) + サブタイトル (1) + MarginBottom (1) +
// ヘルプ行 (1) + MarginTop (1) + 下部余白 (1) = 7
// フィルタ行が表示される場合はさらに +1。
const selectFixedLines = 7

// newSelectModel は探索結果のソリューション一覧から selectModel を初期化する。
// 初期状態ではフィルタなし (全件表示)、どのソリューションも未選択。
func newSelectModel(solutions []scanner.Solution) selectModel {
	filtered := make([]int, len(solutions))
	for i := range solutions {
		filtered[i] = i
	}
	return selectModel{
		solutions: solutions,
		selected:  make(map[int]bool),
		filtered:  filtered,
	}
}

// --- フィルタリング操作 ---

// startFilter はフィルタ入力モードを開始する。
func (m selectModel) startFilter() selectModel {
	m.filtering = true
	return m
}

// clearFilter はフィルタをクリアして全件表示に戻す。
// カーソル位置・スクロールオフセットもリセットする。
func (m selectModel) clearFilter() selectModel {
	m.filtering = false
	m.filter = ""
	m.filtered = make([]int, len(m.solutions))
	for i := range m.solutions {
		m.filtered[i] = i
	}
	m.cursor = 0
	m.offset = 0
	return m
}

// appendFilterChar はフィルタ文字列に 1 文字追加し、フィルタ結果を再計算する。
func (m selectModel) appendFilterChar(r rune) selectModel {
	m.filter += string(r)
	m = m.refilter()
	return m
}

// deleteFilterChar はフィルタ文字列の末尾 1 文字を削除し、フィルタ結果を再計算する。
// フィルタが空の場合はフィルタモードを終了する。
func (m selectModel) deleteFilterChar() selectModel {
	if len(m.filter) == 0 {
		m.filtering = false
		return m
	}
	m.filter = m.filter[:len(m.filter)-1]
	if len(m.filter) == 0 {
		return m.clearFilter()
	}
	m = m.refilter()
	return m
}

// refilter はフィルタ文字列に基づいて filtered スライスを再構築する。
// RelPath に対する大文字小文字無視の部分一致で絞り込む。
// カーソルが範囲外になった場合は先頭にリセットする。
func (m selectModel) refilter() selectModel {
	query := strings.ToLower(m.filter)
	m.filtered = nil
	for i, s := range m.solutions {
		if strings.Contains(strings.ToLower(s.RelPath), query) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
	m.offset = 0
	return m
}

// --- カーソル操作 ---

// cursorUp はカーソルを 1 つ上に移動する。
// 既に先頭にいる場合は何もしない。
func (m selectModel) cursorUp() selectModel {
	if m.cursor > 0 {
		m.cursor--
	}
	return m
}

// cursorDown はカーソルを 1 つ下に移動する。
// 既に末尾にいる場合は何もしない。
func (m selectModel) cursorDown() selectModel {
	if m.cursor < len(m.filtered)-1 {
		m.cursor++
	}
	return m
}

// --- 選択操作 ---

// toggle は現在のカーソル位置のソリューションの選択状態を反転する。
// filtered 経由で実インデックスに変換してから selected を更新する。
func (m selectModel) toggle() selectModel {
	if len(m.filtered) == 0 {
		return m
	}
	realIdx := m.filtered[m.cursor]
	if m.selected[realIdx] {
		delete(m.selected, realIdx)
	} else {
		m.selected[realIdx] = true
	}
	return m
}

// toggleAll はフィルタ結果内の全ソリューションの選択状態を一括切り替えする。
// フィルタ中は表示されている項目のみを対象とする。
// 全て選択済みの場合は全解除し、それ以外の場合は全選択する。
func (m selectModel) toggleAll() selectModel {
	allSelected := true
	for _, idx := range m.filtered {
		if !m.selected[idx] {
			allSelected = false
			break
		}
	}
	if allSelected {
		for _, idx := range m.filtered {
			delete(m.selected, idx)
		}
	} else {
		for _, idx := range m.filtered {
			m.selected[idx] = true
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

// --- 描画 ---

// view はソリューション選択画面の表示文字列を生成する。
// termHeight はターミナルの高さ (行数)。0 の場合はフォールバックとして全件表示する。
//
// 画面構成:
//
//	[タイトル]      "CS Builder"
//	[サブタイトル]   "N 個のソリューションが見つかりました (M 個選択中)"
//	[フィルタ行]     (フィルタモード時のみ) "フィルタ: xxx"
//	[リスト]         ビューポート内のソリューションのみ表示 + スクロールインジケータ
//	[ヘルプ]         キーバインドの説明
func (m selectModel) view(termHeight int) string {
	var b strings.Builder

	title := titleStyle.Render("CS Builder")
	b.WriteString(title + "\n")

	// フィルタ中はフィルタ結果の件数、通常時は全件数を表示
	total := len(m.filtered)
	subtitle := subtitleStyle.Render(
		fmt.Sprintf("%d 個のソリューション (%d 個選択中)", total, len(m.selected)),
	)
	b.WriteString(subtitle + "\n")

	// フィルタ行の表示
	if m.filtering {
		prompt := filterPromptStyle.Render("フィルタ: ")
		text := filterTextStyle.Render(m.filter + "█")
		b.WriteString(prompt + text + "\n")
	}

	// ビューポートの高さを算出する
	vpHeight := m.viewportHeight(termHeight)

	// カーソルが表示範囲内に収まるようオフセットを調整する
	m.adjustOffset(vpHeight)

	// 表示範囲を決定
	endIdx := m.offset + vpHeight
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	// スクロールインジケータの表示判定
	needScroll := len(m.filtered) > vpHeight && vpHeight > 0

	// 表示範囲内の各行を先に組み立て、最大幅を求めてからパディングする。
	// これによりスクロールインジケータの列位置が全行で揃う。
	type rowData struct {
		content string // ANSI エスケープ付きの描画済み行テキスト
		width   int    // 表示上の文字幅 (ANSI エスケープを除く)
	}
	rows := make([]rowData, 0, endIdx-m.offset)
	maxWidth := 0

	for vi := m.offset; vi < endIdx; vi++ {
		realIdx := m.filtered[vi]
		s := m.solutions[realIdx]

		cursor := "  "
		if vi == m.cursor {
			cursor = cursorStyle.Render("▸ ")
		}

		check := "○"
		style := unselectedStyle
		if m.selected[realIdx] {
			check = "●"
			style = selectedStyle
		}

		line := fmt.Sprintf("%s%s %s", cursor, check, style.Render(s.RelPath))
		w := lipgloss.Width(line)
		if w > maxWidth {
			maxWidth = w
		}
		rows = append(rows, rowData{content: line, width: w})
	}

	// 各行を最大幅にパディングしてからスクロールインジケータを付加する
	for i, row := range rows {
		line := row.content
		if needScroll {
			pad := strings.Repeat(" ", maxWidth-row.width)
			indicator := m.scrollIndicator(i, vpHeight, len(m.filtered))
			line += pad + " " + indicator
		}
		b.WriteString(line + "\n")
	}

	// フィルタ結果が 0 件の場合のメッセージ
	if len(m.filtered) == 0 {
		b.WriteString(logStyle.Render("  一致するソリューションがありません") + "\n")
	}

	// ヘルプ行
	helpText := "↑/↓: 移動  space: 選択  a: 全選択  /: フィルタ  enter: ビルド開始 (依存順)  q: 終了"
	if m.filtering {
		helpText = "入力: 絞り込み  backspace: 削除  esc: フィルタ解除  ↑/↓: 移動  space: 選択"
	}
	b.WriteString(helpStyle.Render(helpText))

	return b.String()
}

// viewportHeight はリスト表示に使用できる行数を計算する。
// termHeight が 0 (未取得) の場合はフォールバックとして全件表示できる行数を返す。
func (m selectModel) viewportHeight(termHeight int) int {
	if termHeight <= 0 {
		return len(m.filtered)
	}
	fixed := selectFixedLines
	if m.filtering {
		fixed++
	}
	vp := termHeight - fixed
	if vp < 1 {
		vp = 1
	}
	return vp
}

// adjustOffset はカーソル位置がビューポート内に収まるようオフセットを調整する。
// カーソルが上方に出た場合はオフセットを下げ、下方に出た場合はオフセットを上げる。
func (m *selectModel) adjustOffset(vpHeight int) {
	if vpHeight <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vpHeight {
		m.offset = m.cursor - vpHeight + 1
	}
	maxOffset := len(m.filtered) - vpHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

// scrollIndicator はビューポート内の各行に対するスクロールバー文字を返す。
// viewIdx はビューポート内の行番号 (0-indexed)、total はフィルタ結果の全件数。
//
// offset の範囲 [0, total-vpHeight] を thumbPos の範囲 [0, vpHeight-thumbSize] に
// 線形マッピングすることで、最上部・最下部で正確につまみが端に到達する。
func (m selectModel) scrollIndicator(viewIdx, vpHeight, total int) string {
	if total <= vpHeight || vpHeight <= 0 {
		return ""
	}
	thumbSize := vpHeight * vpHeight / total
	if thumbSize < 1 {
		thumbSize = 1
	}
	maxOffset := total - vpHeight
	thumbPos := 0
	if maxOffset > 0 {
		thumbPos = m.offset * (vpHeight - thumbSize) / maxOffset
	}

	if viewIdx >= thumbPos && viewIdx < thumbPos+thumbSize {
		return scrollThumbStyle.Render("█")
	}
	return scrollIndicatorStyle.Render("│")
}

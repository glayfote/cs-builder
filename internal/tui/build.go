package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"builder/cs-builder/internal/builder"
	"builder/cs-builder/internal/scanner"
)

// buildStatus はビルドキュー内の各ソリューションの状態を表す。
type buildStatus int

const (
	statusPending  buildStatus = iota // ビルド待ち
	statusBuilding                    // ビルド実行中
	statusSuccess                     // ビルド成功
	statusFailure                     // ビルド失敗
)

// buildItem はビルドキュー内の 1 つのソリューションを表す。
// status はビルドの進行に伴って Pending → Building → Success/Failure と遷移する。
type buildItem struct {
	solution scanner.Solution    // ビルド対象のソリューション情報
	status   buildStatus         // 現在のビルドステータス
	result   *builder.BuildResult // ビルド完了後の結果 (未完了時は nil)
}

// buildModel はビルド実行画面の状態を保持する。
// ソリューションを先頭から順次ビルドし、各ソリューションの進捗・結果を管理する。
type buildModel struct {
	items      []buildItem // ビルドキュー (選択されたソリューション群)
	currentIdx int         // 現在ビルド中 (または次にビルドする) アイテムのインデックス
	logLines   []string    // ビルドログ (全行を保持し、表示は末尾から動的行数分)
	done      bool          // 全ソリューションのビルドが完了したか
	offset    int           // アイテム一覧のスクロールオフセット
	startTime time.Time     // ビルド開始時刻 (経過時間表示用)
	elapsed   time.Duration // 確定済みの経過時間 (完了後は固定値)
}

// buildFixedLines はアイテム一覧・ログ以外の固定行数。
// タイトル (1) + MarginBottom (1) + 進捗 (1) + 空行 (1) +
// 空行 (1) + ログヘッダ+罫線 (2) + MarginBottom (1) = 8
// 完了時はサマリ (2行) + ヘルプ (1行) が追加 = +3
const buildFixedLines = 8

// newBuildModel は選択されたソリューション群からビルドキューを初期化する。
// 全アイテムの初期状態は statusPending。
func newBuildModel(solutions []scanner.Solution) buildModel {
	items := make([]buildItem, len(solutions))
	for i, s := range solutions {
		items[i] = buildItem{solution: s, status: statusPending}
	}
	return buildModel{
		items:     items,
		startTime: time.Now(),
	}
}

// startNext は currentIdx が指すアイテムのステータスを statusBuilding に遷移させる。
// ビルド開始前に呼び出して、画面表示にスピナーを反映させる。
// currentIdx がキューの範囲外の場合は何もしない。
func (m *buildModel) startNext() {
	if m.currentIdx < len(m.items) {
		m.items[m.currentIdx].status = statusBuilding
	}
}

// completeCurrent は現在ビルド中のアイテムにビルド結果を設定し、
// ステータスを Success/Failure に遷移させる。
// その後 currentIdx を進め、全アイテム完了なら done を true にする。
func (m *buildModel) completeCurrent(result builder.BuildResult) {
	if m.currentIdx >= len(m.items) {
		return
	}
	m.items[m.currentIdx].result = &result
	if result.Success {
		m.items[m.currentIdx].status = statusSuccess
	} else {
		m.items[m.currentIdx].status = statusFailure
	}

	m.currentIdx++
	if m.currentIdx >= len(m.items) {
		m.done = true
		m.elapsed = time.Since(m.startTime)
	}
}

// appendLog はビルドログに 1 行追加する。
// ログは全行保持し、表示時にターミナル高さに応じて末尾から切り出す。
func (m *buildModel) appendLog(line string) {
	m.logLines = append(m.logLines, line)
}

// view はビルド進捗画面の表示文字列を生成する。
// spinner 引数にはアニメーション用の現在のスピナーフレーム文字を渡す。
// ビルド完了後 (stateDone) は空文字列を渡すことでスピナーを非表示にする。
// termHeight はターミナルの高さ (行数)。
//
// アイテム一覧とログ領域にターミナル高さの残りを半分ずつ割り当てる。
// アイテム数が少ない場合はログ領域を広く使う。
//
// 画面構成:
//
//	[タイトル]     "CS Builder - ビルド"
//	[進捗]         "進捗: N / M"
//	[アイテム一覧]  スクロール可能、ビルド中アイテムを追従
//	[ログ区切り]   ───── ビルドログ ─────
//	[ログ]         ターミナル高さに応じた動的行数
//	[サマリ]       (完了時のみ) 成功/失敗の件数表示
func (m buildModel) view(spinner string, termHeight int) string {
	var b strings.Builder

	title := titleStyle.Render("CS Builder - ビルド")
	b.WriteString(title + "\n")

	// 成功・失敗の件数を集計して進捗表示に使用する
	succeeded := 0
	failed := 0
	for _, item := range m.items {
		switch item.status {
		case statusSuccess:
			succeeded++
		case statusFailure:
			failed++
		}
	}
	completed := succeeded + failed
	elapsed := m.elapsed
	if !m.done {
		elapsed = time.Since(m.startTime)
	}
	progress := progressStyle.Render(
		fmt.Sprintf("進捗: %d / %d  (%s)", completed, len(m.items), elapsed.Truncate(time.Second)),
	)
	b.WriteString(progress + "\n\n")

	// アイテム一覧とログの表示行数を計算する
	itemVP, logVP := m.layoutHeights(termHeight)

	// ビルド中のアイテムが表示範囲に収まるようオフセットを調整する
	m.adjustOffset(itemVP)

	// アイテム一覧のスクロール描画
	endIdx := m.offset + itemVP
	if endIdx > len(m.items) {
		endIdx = len(m.items)
	}
	needScroll := len(m.items) > itemVP && itemVP > 0

	// 表示範囲内の各行を先に組み立て、最大幅を求めてからパディングする。
	// これによりスクロールインジケータの列位置が全行で揃う。
	type rowData struct {
		content string
		width   int
	}
	rows := make([]rowData, 0, endIdx-m.offset)
	maxWidth := 0

	for vi := m.offset; vi < endIdx; vi++ {
		item := m.items[vi]
		icon := statusIcon(item.status, spinner)
		name := item.solution.RelPath
		line := fmt.Sprintf("  %s %s", icon, name)
		if item.result != nil {
			line += fmt.Sprintf("  (%s)", item.result.Duration.Truncate(100*1e6))
		}
		w := lipgloss.Width(line)
		if w > maxWidth {
			maxWidth = w
		}
		rows = append(rows, rowData{content: line, width: w})
	}

	for i, row := range rows {
		line := row.content
		if needScroll {
			pad := strings.Repeat(" ", maxWidth-row.width)
			indicator := m.scrollIndicator(i, itemVP, len(m.items))
			line += pad + " " + indicator
		}
		b.WriteString(line + "\n")
	}

	// ビルドログセクション
	b.WriteString("\n")
	b.WriteString(headerBorder.Render("ビルドログ") + "\n")

	// ログの末尾から logVP 行分を表示する
	logStart := 0
	if len(m.logLines) > logVP {
		logStart = len(m.logLines) - logVP
	}
	for i := logStart; i < len(m.logLines); i++ {
		b.WriteString(logStyle.Render(m.logLines[i]) + "\n")
	}

	// 全ビルド完了時のサマリ表示
	if m.done {
		b.WriteString("\n")
		summary := fmt.Sprintf("完了: %d 成功, %d 失敗", succeeded, failed)
		if failed > 0 {
			b.WriteString(failureStyle.Render(summary) + "\n")
		} else {
			b.WriteString(successStyle.Render(summary) + "\n")
		}
		b.WriteString(helpStyle.Render("enter/q: 終了"))
	}

	return b.String()
}

// layoutHeights はターミナル高さからアイテム一覧とログ領域の表示行数を計算する。
// アイテム数が少ない場合はログ領域に余り行を割り当てる。
// termHeight が 0 の場合はフォールバックとして固定値を返す。
func (m buildModel) layoutHeights(termHeight int) (itemVP, logVP int) {
	if termHeight <= 0 {
		return len(m.items), 20
	}

	fixed := buildFixedLines
	if m.done {
		fixed += 3
	}
	available := termHeight - fixed
	if available < 2 {
		available = 2
	}

	// アイテム一覧は最大でアイテム数分、残りをログに割り当てる
	itemVP = available / 2
	if itemVP > len(m.items) {
		itemVP = len(m.items)
	}
	if itemVP < 1 {
		itemVP = 1
	}
	logVP = available - itemVP
	if logVP < 1 {
		logVP = 1
	}
	return itemVP, logVP
}

// adjustOffset はビルド中のアイテムが表示範囲内に収まるようオフセットを調整する。
func (m *buildModel) adjustOffset(vpHeight int) {
	if vpHeight <= 0 {
		return
	}
	// ビルド中 or 最後に完了したアイテムの位置を追従対象とする
	target := m.currentIdx
	if target >= len(m.items) {
		target = len(m.items) - 1
	}
	if target < 0 {
		return
	}

	if target < m.offset {
		m.offset = target
	}
	if target >= m.offset+vpHeight {
		m.offset = target - vpHeight + 1
	}
	maxOffset := len(m.items) - vpHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

// scrollIndicator はビューポート内の各行に対するスクロールバー文字を返す。
//
// offset の範囲 [0, total-vpHeight] を thumbPos の範囲 [0, vpHeight-thumbSize] に
// 線形マッピングすることで、最上部・最下部で正確につまみが端に到達する。
func (m buildModel) scrollIndicator(viewIdx, vpHeight, total int) string {
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

// statusIcon はビルドステータスに対応するアイコン文字列を返す。
// statusBuilding の場合は spinner 引数のフレーム文字がそのまま使用される。
//
// アイコン一覧:
//
//	statusPending  → ○ (グレー: 待機中)
//	statusBuilding → スピナー文字 (マゼンタ: ビルド中)
//	statusSuccess  → ✓ (緑: 成功)
//	statusFailure  → ✗ (赤: 失敗)
func statusIcon(s buildStatus, spinner string) string {
	switch s {
	case statusPending:
		return unselectedStyle.Render("○")
	case statusBuilding:
		return spinnerStyle.Render(spinner)
	case statusSuccess:
		return successStyle.Render("✓")
	case statusFailure:
		return failureStyle.Render("✗")
	default:
		return " "
	}
}

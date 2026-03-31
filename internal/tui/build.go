package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"builder/cs-builder/internal/builder"
	"builder/cs-builder/internal/depgraph"
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
// ソリューションを依存レベル単位でグループ化し、同一レベル内で並列ビルドを行う。
// レベル間は直列で処理し、依存先が先にビルドされることを保証する。
type buildModel struct {
	items        []buildItem    // ビルドキュー (依存順にソート済み)
	levels       [][]int        // levels[i] = レベル i に属するアイテムの items 内インデックス
	currentLevel int            // 現在処理中のレベル
	maxParallel  int            // 最大並列ビルド数 (0 = 無制限)
	activeCount  int            // 現在実行中のビルド数
	logLines     []string       // ビルドログ (全行を保持し、表示は末尾から動的行数分)
	done         bool           // 全ソリューションのビルドが完了したか
	offset       int            // アイテム一覧のスクロールオフセット
	startTime    time.Time      // ビルド開始時刻 (経過時間表示用)
	elapsed      time.Duration  // 確定済みの経過時間 (完了後は固定値)
}

// buildFixedLines はアイテム一覧・ログ以外の固定行数。
// タイトル (1) + MarginBottom (1) + 進捗 (1) + 空行 (1) +
// 空行 (1) + ログヘッダ+罫線 (2) + MarginBottom (1) = 8
// 完了時はサマリ (2行) + ヘルプ (1行) が追加 = +3
const buildFixedLines = 8

// newBuildModel は依存順にソートされたノード群からビルドキューを初期化する。
// ノードの Level フィールドに基づいてレベルグループを構築する。
func newBuildModel(nodes []*depgraph.Node, maxParallel int) buildModel {
	items := make([]buildItem, len(nodes))
	levelMax := 0
	for i, n := range nodes {
		items[i] = buildItem{solution: n.Solution, status: statusPending}
		if n.Level > levelMax {
			levelMax = n.Level
		}
	}

	levels := make([][]int, levelMax+1)
	for i, n := range nodes {
		levels[n.Level] = append(levels[n.Level], i)
	}

	return buildModel{
		items:       items,
		levels:      levels,
		maxParallel: maxParallel,
		startTime:   time.Now(),
	}
}

// pendingInCurrentLevel は現在レベルの未開始アイテムのインデックスを返す。
func (m *buildModel) pendingInCurrentLevel() []int {
	if m.currentLevel >= len(m.levels) {
		return nil
	}
	var pending []int
	for _, idx := range m.levels[m.currentLevel] {
		if m.items[idx].status == statusPending {
			pending = append(pending, idx)
		}
	}
	return pending
}

// completeItem は指定インデックスのアイテムにビルド結果を設定し、
// ステータスを Success/Failure に遷移させる。
// レベル内の全アイテムが完了した場合は次のレベルに進む。
// 全レベル完了なら done を true にする。
func (m *buildModel) completeItem(idx int, result builder.BuildResult) {
	if idx < 0 || idx >= len(m.items) {
		return
	}
	m.items[idx].result = &result
	if result.Success {
		m.items[idx].status = statusSuccess
	} else {
		m.items[idx].status = statusFailure
	}
	m.activeCount--

	if m.isLevelDone(m.currentLevel) {
		m.currentLevel++
		if m.currentLevel >= len(m.levels) {
			m.done = true
			m.elapsed = time.Since(m.startTime)
		}
	}
}

// isLevelDone は指定レベルの全アイテムがビルド済み (成功 or 失敗) かを返す。
func (m *buildModel) isLevelDone(level int) bool {
	if level >= len(m.levels) {
		return true
	}
	for _, idx := range m.levels[level] {
		s := m.items[idx].status
		if s == statusPending || s == statusBuilding {
			return false
		}
	}
	return true
}

// scrollTarget はスクロール追従対象のアイテムインデックスを返す。
// ビルド中のアイテムのうち最後のものを優先し、
// なければ最後に完了したアイテムにフォールバックする。
func (m *buildModel) scrollTarget() int {
	last := -1
	for i, item := range m.items {
		if item.status == statusBuilding {
			last = i
		}
	}
	if last >= 0 {
		return last
	}
	for i := len(m.items) - 1; i >= 0; i-- {
		if m.items[i].status == statusSuccess || m.items[i].status == statusFailure {
			return i
		}
	}
	return 0
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
//	[進捗]         "進捗: N / M  (P 並列)  (Xs)"
//	[アイテム一覧]  スクロール可能、ビルド中アイテムを追従
//	[ログ区切り]   ───── ビルドログ ─────
//	[ログ]         ターミナル高さに応じた動的行数
//	[サマリ]       (完了時のみ) 成功/失敗の件数表示
func (m buildModel) view(spinner string, termHeight int) string {
	var b strings.Builder

	title := titleStyle.Render("CS Builder - ビルド")
	b.WriteString(title + "\n")

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

	parallelInfo := ""
	if m.activeCount > 1 {
		parallelInfo = fmt.Sprintf("  (%d 並列)", m.activeCount)
	}
	progress := progressStyle.Render(
		fmt.Sprintf("進捗: %d / %d%s  (%s)", completed, len(m.items), parallelInfo, elapsed.Truncate(time.Second)),
	)
	b.WriteString(progress + "\n\n")

	itemVP, logVP := m.layoutHeights(termHeight)

	m.adjustOffset(itemVP)

	endIdx := m.offset + itemVP
	if endIdx > len(m.items) {
		endIdx = len(m.items)
	}
	needScroll := len(m.items) > itemVP && itemVP > 0

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

	b.WriteString("\n")
	b.WriteString(headerBorder.Render("ビルドログ") + "\n")

	logStart := 0
	if len(m.logLines) > logVP {
		logStart = len(m.logLines) - logVP
	}
	for i := logStart; i < len(m.logLines); i++ {
		b.WriteString(logStyle.Render(m.logLines[i]) + "\n")
	}

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
	target := m.scrollTarget()
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

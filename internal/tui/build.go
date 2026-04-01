package tui

import (
	"fmt"
	"sort"
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
// 選択サブグラフ上で「準備完了 (選択内前提がすべて完了)」なアイテムから起動し、
// 選択内の他ノードから参照されているものを優先し、並列枠に余りがあれば低優先も起動する。
type buildModel struct {
	items                 []buildItem // ビルドキュー (itemIdx はビルド Cmd と固定対応)
	displayOrder          []int       // 画面上の行 i に items[displayOrder[i]] を表示 (refreshDisplayOrder で更新)
	remainingInBatchDeps  []int       // 選択内前提のうち未完了の個数
	dependents            [][]int     // 前提 i 完了時に remaining を減らす従属アイテムのインデックス
	neededBySelected      []bool      // 選択内の誰かがこの出力を内部参照している (高優先)
	maxParallel           int         // 最大並列ビルド数 (0 = 無制限)
	activeCount           int         // 現在実行中のビルド数
	logLines              []string    // ビルドログ (全行を保持し、表示は末尾から動的行数分)
	done                  bool        // 全ソリューションのビルドが完了したか
	offset                int         // アイテム一覧のスクロールオフセット (displayOrder 行基準)
	startTime             time.Time   // ビルド開始時刻 (経過時間表示用)
	elapsed               time.Duration // 確定済みの経過時間 (完了後は固定値)
}

// buildFixedLines はアイテム一覧・ログ以外の固定行数。
// タイトル (1) + MarginBottom (1) + 進捗 (1) + 空行 (1) +
// 空行 (1) + ログヘッダ+罫線 (2) + MarginBottom (1) = 8
// 完了時はサマリ (2行) + ヘルプ (1行) が追加 = +3
const buildFixedLines = 8

// newBuildModel は依存順にソートされたノード群からビルドキューを初期化する。
// g が nil のときは選択内依存なし (全件すぐ準備完了・優先度差なし) として扱う。
func newBuildModel(g *depgraph.Graph, nodes []*depgraph.Node, maxParallel int) buildModel {
	if len(nodes) == 0 {
		return buildModel{maxParallel: maxParallel, startTime: time.Now()}
	}

	items := make([]buildItem, len(nodes))
	for i, n := range nodes {
		items[i] = buildItem{solution: n.Solution, status: statusPending}
	}

	var remaining []int
	var dependents [][]int
	if g != nil {
		remaining, dependents = g.ScheduleFromSorted(nodes)
	}
	if remaining == nil {
		remaining = make([]int, len(nodes))
		dependents = make([][]int, len(nodes))
	}
	needed := make([]bool, len(nodes))
	for i := range nodes {
		needed[i] = len(dependents[i]) > 0
	}

	bm := buildModel{
		items:                items,
		remainingInBatchDeps: remaining,
		dependents:           dependents,
		neededBySelected:     needed,
		maxParallel:          maxParallel,
		startTime:            time.Now(),
	}
	bm.refreshDisplayOrder()
	return bm
}

// displayRowGroup は画面上の縦並び用のグループ番号 (小さいほど上に表示)。
func displayRowGroup(m *buildModel, i int) int {
	switch m.items[i].status {
	case statusBuilding:
		return 0
	case statusPending:
		if m.remainingInBatchDeps[i] == 0 {
			if m.neededBySelected[i] {
				return 1 // 準備完了・高優先
			}
			return 2 // 準備完了・低優先
		}
		return 3 // 前提待ち
	case statusSuccess, statusFailure:
		return 4
	default:
		return 4
	}
}

// refreshDisplayOrder はスケジューラと整合した優先度で表示行順 displayOrder を再構築する。
func (m *buildModel) refreshDisplayOrder() {
	n := len(m.items)
	if n == 0 {
		m.displayOrder = nil
		return
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		ia, ib := order[a], order[b]
		ga, gb := displayRowGroup(m, ia), displayRowGroup(m, ib)
		if ga != gb {
			return ga < gb
		}
		switch ga {
		case 0, 1, 2, 4: // ビルド中 / 準備完了 / 完了 は RelPath で安定順
			return m.items[ia].solution.RelPath < m.items[ib].solution.RelPath
		case 3: // 前提待ち: 残り前提が少ないほど上、同率は高優先→RelPath
			ra, rb := m.remainingInBatchDeps[ia], m.remainingInBatchDeps[ib]
			if ra != rb {
				return ra < rb
			}
			if m.neededBySelected[ia] != m.neededBySelected[ib] {
				return m.neededBySelected[ia]
			}
			return m.items[ia].solution.RelPath < m.items[ib].solution.RelPath
		default:
			return m.items[ia].solution.RelPath < m.items[ib].solution.RelPath
		}
	})
	m.displayOrder = order
}

// readyPendingIndices は選択内前提がすべて完了済みで、まだ Pending のインデックスを返す。
func (m *buildModel) readyPendingIndices() []int {
	var out []int
	for i := range m.items {
		if m.items[i].status != statusPending {
			continue
		}
		if m.remainingInBatchDeps[i] != 0 {
			continue
		}
		out = append(out, i)
	}
	return out
}

// pickNextToStart は maxParallel と activeCount に応じて、今回起動するインデックスを返す。
// 高優先 (neededBySelected) を RelPath 順で先に埋め、余り枠で低優先を同様に取る。
func (m *buildModel) pickNextToStart() []int {
	ready := m.readyPendingIndices()
	if len(ready) == 0 {
		return nil
	}

	var high, low []int
	for _, idx := range ready {
		if m.neededBySelected[idx] {
			high = append(high, idx)
		} else {
			low = append(low, idx)
		}
	}
	sort.Slice(high, func(i, j int) bool {
		return m.items[high[i]].solution.RelPath < m.items[high[j]].solution.RelPath
	})
	sort.Slice(low, func(i, j int) bool {
		return m.items[low[i]].solution.RelPath < m.items[low[j]].solution.RelPath
	})

	slots := len(ready)
	if m.maxParallel > 0 {
		slots = m.maxParallel - m.activeCount
		if slots < 0 {
			slots = 0
		}
	}
	if slots == 0 {
		return nil
	}

	var out []int
	for _, idx := range high {
		if len(out) >= slots {
			break
		}
		out = append(out, idx)
	}
	for _, idx := range low {
		if len(out) >= slots {
			break
		}
		out = append(out, idx)
	}
	return out
}

// completeItem は指定インデックスのアイテムにビルド結果を設定し、
// ステータスを Success/Failure に遷移させる。
// 従属アイテムの remainingInBatchDeps を減らし、全アイテム完了なら done を true にする。
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

	for _, j := range m.dependents[idx] {
		if j >= 0 && j < len(m.remainingInBatchDeps) && m.remainingInBatchDeps[j] > 0 {
			m.remainingInBatchDeps[j]--
		}
	}

	if m.allItemsFinished() {
		m.done = true
		m.elapsed = time.Since(m.startTime)
	}
	m.refreshDisplayOrder()
}

func (m *buildModel) allItemsFinished() bool {
	for i := range m.items {
		s := m.items[i].status
		if s == statusPending || s == statusBuilding {
			return false
		}
	}
	return true
}

// scrollTarget はスクロール追従対象の displayOrder 上の行インデックスを返す。
// ビルド中のアイテムのうち最後のものを優先し、
// なければ最後に完了したアイテムにフォールバックする。
func (m *buildModel) scrollTarget() int {
	targetItem := 0
	lastBuilding := -1
	for i, item := range m.items {
		if item.status == statusBuilding {
			lastBuilding = i
		}
	}
	if lastBuilding >= 0 {
		targetItem = lastBuilding
	} else {
		for i := len(m.items) - 1; i >= 0; i-- {
			if m.items[i].status == statusSuccess || m.items[i].status == statusFailure {
				targetItem = i
				break
			}
		}
	}
	for row, idx := range m.displayOrder {
		if idx == targetItem {
			return row
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

	if !m.done {
		m.adjustOffset(itemVP)
	} else {
		m.clampBuildListOffset(itemVP)
	}

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

	displayOrder := m.displayOrder
	if len(displayOrder) != len(m.items) {
		displayOrder = make([]int, len(m.items))
		for i := range displayOrder {
			displayOrder[i] = i
		}
	}

	for vi := m.offset; vi < endIdx; vi++ {
		itemIdx := displayOrder[vi]
		item := m.items[itemIdx]
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
		help := "enter/q: 終了"
		if needScroll {
			help += "  ↑↓ jk  PgUp/PgDn  Home/End: 一覧"
		}
		b.WriteString(helpStyle.Render(help))
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
	m.clampBuildListOffset(vpHeight)
}

// clampBuildListOffset は一覧の offset を有効範囲に収める。
func (m *buildModel) clampBuildListOffset(vpHeight int) {
	if vpHeight <= 0 {
		return
	}
	maxOffset := len(m.items) - vpHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

// scrollBuildList は一覧の先頭行オフセットを delta 分動かす (完了後の手動スクロール用)。
func (m *buildModel) scrollBuildList(delta, vpHeight int) {
	m.offset += delta
	m.clampBuildListOffset(vpHeight)
}

// scrollBuildListHome は一覧の先頭へ、End は末尾付近へスクロールする。
func (m *buildModel) scrollBuildListHome(vpHeight int) {
	m.offset = 0
	m.clampBuildListOffset(vpHeight)
}

func (m *buildModel) scrollBuildListEnd(vpHeight int) {
	maxOffset := len(m.items) - vpHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.offset = maxOffset
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

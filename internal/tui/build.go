package tui

import (
	"fmt"
	"strings"

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
	logLines   []string    // ビルドログの直近 N 行 (リングバッファ的に古い行を破棄)
	maxLog     int         // logLines に保持する最大行数
	done       bool        // 全ソリューションのビルドが完了したか
}

// newBuildModel は選択されたソリューション群からビルドキューを初期化する。
// 全アイテムの初期状態は statusPending。
// maxLog は TUI に表示するビルドログの最大行数で、デフォルト 20 行。
func newBuildModel(solutions []scanner.Solution) buildModel {
	items := make([]buildItem, len(solutions))
	for i, s := range solutions {
		items[i] = buildItem{solution: s, status: statusPending}
	}
	return buildModel{
		items:  items,
		maxLog: 20,
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

	// 次のアイテムに進む。全て完了した場合は done フラグをセットする。
	m.currentIdx++
	if m.currentIdx >= len(m.items) {
		m.done = true
	}
}

// appendLog はビルドログに 1 行追加する。
// maxLog を超えた場合は古い行を先頭から破棄し、直近の maxLog 行のみ保持する。
// これにより TUI の表示が溢れることを防ぐ。
func (m *buildModel) appendLog(line string) {
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > m.maxLog {
		m.logLines = m.logLines[len(m.logLines)-m.maxLog:]
	}
}

// view はビルド進捗画面の表示文字列を生成する。
// spinner 引数にはアニメーション用の現在のスピナーフレーム文字を渡す。
// ビルド完了後 (stateDone) は空文字列を渡すことでスピナーを非表示にする。
//
// 画面構成:
//
//	[タイトル]     "CS Builder - ビルド"
//	[進捗]         "進捗: N / M"
//	[アイテム一覧]  各ソリューションのステータスアイコン + 相対パス + 所要時間
//	[ログ区切り]   ───── ビルドログ ─────
//	[ログ]         直近 maxLog 行のビルドログ出力
//	[サマリ]       (完了時のみ) 成功/失敗の件数表示
func (m buildModel) view(spinner string) string {
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
	progress := progressStyle.Render(
		fmt.Sprintf("進捗: %d / %d", completed, len(m.items)),
	)
	b.WriteString(progress + "\n\n")

	// 各ソリューションのステータスを 1 行ずつ描画する
	for _, item := range m.items {
		icon := statusIcon(item.status, spinner)
		name := item.solution.RelPath
		line := fmt.Sprintf("  %s %s", icon, name)
		// ビルド完了済みのアイテムには所要時間を付記する (100ms 単位で丸め)
		if item.result != nil {
			line += fmt.Sprintf("  (%s)", item.result.Duration.Truncate(100*1e6))
		}
		b.WriteString(line + "\n")
	}

	// ビルドログセクション
	b.WriteString("\n")
	b.WriteString(headerBorder.Render("ビルドログ") + "\n")

	for _, line := range m.logLines {
		b.WriteString(logStyle.Render(line) + "\n")
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

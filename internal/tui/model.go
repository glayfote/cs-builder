package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"builder/cs-builder/internal/builder"
	"builder/cs-builder/internal/scanner"
)

// state はアプリケーション全体の画面状態を表す。
// 画面遷移は一方向で、Scanning → Selecting → Building → Done の順に進む。
type state int

const (
	stateScanning  state = iota // .sln ファイルの探索中 (非同期)
	stateSelecting              // ユーザによるソリューション選択画面
	stateBuilding               // 選択されたソリューションを順次ビルド中
	stateDone                   // 全ビルド完了、サマリ表示中
)

// Model は Bubble Tea のトップレベルモデル。
// tea.Model インターフェース (Init, Update, View) を実装する。
//
// 各画面の描画・操作は selectModel / buildModel に委譲し、
// Model は画面遷移の制御と非同期コマンドの発行を担当する。
//
// Err フィールドはエクスポートされており、TUI 終了後に
// cmd/root.go がエラーの有無を確認するために使用する。
type Model struct {
	state     state              // 現在の画面状態
	baseDir   string             // .sln を探索するベースディレクトリ
	buildOpts builder.BuildOption // ビルドコマンドのオプション (コマンド名、構成)

	sel   selectModel // ソリューション選択画面のサブモデル
	build buildModel  // ビルド実行画面のサブモデル

	spinnerFrames []string // スピナーアニメーションのフレーム文字列
	spinnerIdx    int      // 現在表示中のスピナーフレームのインデックス

	Err error // TUI 内部で発生したエラー (スキャン失敗等)。外部から参照可能。
}

// NewModel は TUI モデルを初期化する。
// cmd/root.go から呼ばれ、CLI フラグの値がそのまま渡される。
//
// 引数:
//   - baseDir  : .sln ファイルを探索するルートディレクトリのパス
//   - buildCmd : ビルドコマンド名 ("dotnet" または "msbuild")
//   - config   : ビルド構成 ("Debug" または "Release")
//
// 初期状態は stateScanning で、Init() により非同期スキャンが開始される。
func NewModel(baseDir, buildCmd, config string) Model {
	return Model{
		state:   stateScanning,
		baseDir: baseDir,
		buildOpts: builder.BuildOption{
			Command:       buildCmd,
			Configuration: config,
		},
		// Braille パターンによるスピナーアニメーション (10 フレーム)
		spinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// --- Bubble Tea メッセージ型 ---

// scanDoneMsg は非同期 .sln スキャンの完了を通知するメッセージ。
// err が非 nil の場合はスキャン失敗を意味する。
type scanDoneMsg struct {
	solutions []scanner.Solution // 見つかったソリューション一覧
	err       error              // スキャン中に発生したエラー (正常時は nil)
}

// tickMsg はスピナーアニメーションの更新タイミングを通知するメッセージ。
// 80ms 間隔で定期的に送信される。
type tickMsg struct{}

// buildBatchMsg はビルド完了時にログ行と結果をまとめて返すメッセージ。
//
// Bubble Tea の Cmd は単一の Msg しか返せないため、
// ビルド中の個別ログ行をリアルタイムに送信することはできない。
// 代わりに、ビルド完了時に全ログ行をまとめて logs に格納し、
// 結果と一緒に 1 つのメッセージとして返す方式を採用している。
type buildBatchMsg struct {
	logs   []string            // ビルド中に出力された全ログ行
	result builder.BuildResult // ビルドの最終結果
}

// tickCmd は 80ms 後に tickMsg を送信する Bubble Tea コマンドを返す。
// スピナーアニメーションのフレーム更新に使用される。
// Update で tickMsg を受信するたびに再度 tickCmd() を返すことで、
// アプリケーション終了までアニメーションが継続する。
func tickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// --- tea.Model インターフェース実装 ---

// Init は Bubble Tea がモデルを初期化する際に呼ばれる。
// .sln スキャンの非同期開始とスピナーアニメーションの開始を同時に行う。
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.scanCmd(), tickCmd())
}

// Update は Bubble Tea がメッセージを受信するたびに呼ばれる。
// メッセージの型に応じて適切なハンドラに処理を委譲する。
//
// 処理するメッセージ:
//   - tea.KeyMsg     : キー入力 → handleKey で画面状態別に処理
//   - scanDoneMsg    : スキャン完了 → handleScanDone で選択画面に遷移
//   - buildBatchMsg  : ビルド完了 → handleBuildBatch でログ更新と次のビルド開始
//   - tickMsg        : スピナー更新 → フレームインデックスを進める
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case scanDoneMsg:
		return m.handleScanDone(msg)
	case buildBatchMsg:
		m, cmd := m.handleBuildBatch(msg)
		return m, cmd
	case tickMsg:
		m.spinnerIdx = (m.spinnerIdx + 1) % len(m.spinnerFrames)
		return m, tickCmd()
	}
	return m, nil
}

// View は現在の画面状態に応じた表示文字列を返す。
// Bubble Tea が毎フレーム呼び出し、ターミナルに描画する。
//
// 画面状態と描画内容:
//   - stateScanning  : タイトル + スピナー + "探索中..." メッセージ
//   - stateSelecting : selectModel.view() による選択リスト
//   - stateBuilding  : buildModel.view() による進捗表示 (スピナー付き)
//   - stateDone      : buildModel.view() によるサマリ表示 (スピナーなし)
func (m Model) View() string {
	switch m.state {
	case stateScanning:
		spinner := m.spinnerFrames[m.spinnerIdx]
		return titleStyle.Render("CS Builder") + "\n\n" +
			spinnerStyle.Render(spinner) + " .sln ファイルを探索中...\n"
	case stateSelecting:
		return m.sel.view()
	case stateBuilding:
		spinner := m.spinnerFrames[m.spinnerIdx]
		return m.build.view(spinner)
	case stateDone:
		return m.build.view("")
	}
	return ""
}

// --- キー入力ハンドラ ---

// handleKey はキー入力メッセージを画面状態に応じて振り分ける。
// Ctrl+C はどの画面でもアプリケーションを即座に終了する。
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Ctrl+C は全画面共通で即座に終了
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.state {
	case stateSelecting:
		return m.handleSelectKey(key)
	case stateDone:
		if key == "enter" || key == "q" {
			return m, tea.Quit
		}
	}
	return m, nil
}

// handleSelectKey はソリューション選択画面でのキー入力を処理する。
//
// キーバインド:
//   - up/k   : カーソルを 1 つ上に移動
//   - down/j : カーソルを 1 つ下に移動
//   - space  : カーソル位置のソリューションの選択をトグル
//   - a      : 全ソリューションの選択をトグル (全選択 ↔ 全解除)
//   - enter  : 選択中のソリューションのビルドを開始 (未選択時は無視)
//   - q      : アプリケーションを終了
func (m Model) handleSelectKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.sel = m.sel.cursorUp()
	case "down", "j":
		m.sel = m.sel.cursorDown()
	case " ":
		m.sel = m.sel.toggle()
	case "a":
		m.sel = m.sel.toggleAll()
	case "enter":
		// 未選択状態では Enter を無視し、選択画面にとどまる
		if !m.sel.hasSelection() {
			return m, nil
		}
		// 選択されたソリューションでビルドキューを初期化し、
		// 最初のソリューションのビルドを開始する
		selected := m.sel.selectedSolutions()
		m.build = newBuildModel(selected)
		m.state = stateBuilding
		m.build.startNext()
		return m, m.runBuildCmd()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// --- 非同期コマンド ---

// scanCmd は .sln ファイルの非同期スキャンを実行する Bubble Tea コマンドを返す。
// バックグラウンドで scanner.Scan を呼び出し、結果を scanDoneMsg として送信する。
func (m Model) scanCmd() tea.Cmd {
	baseDir := m.baseDir
	return func() tea.Msg {
		solutions, err := scanner.Scan(baseDir)
		return scanDoneMsg{solutions: solutions, err: err}
	}
}

// handleScanDone はスキャン完了メッセージを処理する。
//
// 3 つのケースを処理する:
//  1. スキャンエラー: Err にセットして TUI を終了
//  2. ソリューションが 0 件: Done 画面に直行 (空のサマリ表示)
//  3. 1 件以上: selectModel を初期化して選択画面に遷移
func (m Model) handleScanDone(msg scanDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.Err = msg.err
		return m, tea.Quit
	}
	if len(msg.solutions) == 0 {
		m.state = stateDone
		m.build = newBuildModel(nil)
		return m, nil
	}
	m.sel = newSelectModel(msg.solutions)
	m.state = stateSelecting
	return m, nil
}

// runBuildCmd は現在のビルドキューの先頭アイテムに対してビルドを実行する
// Bubble Tea コマンドを返す。
//
// 実行の流れ:
//  1. builder.Build をゴルーチンで非同期実行する
//  2. ビルド中のログ行は logCh チャネル経由で収集する
//  3. logCh が close されたら (ビルド出力終了)、doneCh から結果を受け取る
//  4. 全ログ行と結果を buildBatchMsg にまとめて返す
//
// 注意: Bubble Tea の Cmd は 1 つの Msg しか返せないため、
// ビルド中のリアルタイムログ更新はできず、完了時に一括で反映される。
func (m Model) runBuildCmd() tea.Cmd {
	idx := m.build.currentIdx
	if idx >= len(m.build.items) {
		return nil
	}
	item := m.build.items[idx]
	opts := m.buildOpts
	return func() tea.Msg {
		logCh := make(chan string, 64)
		doneCh := make(chan builder.BuildResult, 1)

		// ビルドをバックグラウンドのゴルーチンで実行する。
		// logCh にログ行が逐次送信され、完了後に close される。
		go func() {
			result := builder.Build(context.Background(), item.solution.AbsPath, opts, logCh)
			close(logCh)
			doneCh <- result
		}()

		// logCh から全ログ行を消費して蓄積する。
		// チャネルが close されるまで (= ビルドの全出力が終わるまで) ブロックする。
		var logLines []string
		for line := range logCh {
			logLines = append(logLines, line)
		}

		result := <-doneCh
		return buildBatchMsg{logs: logLines, result: result}
	}
}

// handleBuildBatch はビルド完了メッセージを処理する。
//
// 処理の流れ:
//  1. ログ行を buildModel に追加する (画面に表示される)
//  2. 現在のアイテムを完了させ、結果を記録する
//  3. 全アイテム完了なら Done 画面に遷移
//  4. 未完了なら次のアイテムのビルドを開始する
func (m Model) handleBuildBatch(msg buildBatchMsg) (Model, tea.Cmd) {
	for _, line := range msg.logs {
		m.build.appendLog(line)
	}
	m.build.completeCurrent(msg.result)
	if m.build.done {
		m.state = stateDone
		return m, nil
	}
	m.build.startNext()
	return m, m.runBuildCmd()
}

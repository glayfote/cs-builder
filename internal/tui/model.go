package tui

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"builder/cs-builder/internal/artifact"
	"builder/cs-builder/internal/builder"
	"builder/cs-builder/internal/depgraph"
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
	state       state              // 現在の画面状態
	projectRoot string             // プロジェクトルート (RelPath / shared_dll_dir の基準)
	scanRoots   []string           // スキャン対象サブディレクトリ (projectRoot からの相対、空なら全体)
	buildOpts   builder.BuildOption // ビルドコマンドのオプション (コマンド名、構成)
	maxParallel int                // 最大並列ビルド数 (0=無制限)

	scanExcludes []string           // スキャン時の追加除外パターン (.cs-builder.toml の scan.exclude)
	dllDirMap    map[string]string  // scan root path → コピー先絶対パス (空マップならコピー無効)
	buildLog     io.Writer          // MSBuild/dotnet 全文ログ (nil ならファイル出力しない)

	width  int // ターミナルの幅 (tea.WindowSizeMsg で更新)
	height int // ターミナルの高さ (tea.WindowSizeMsg で更新)

	graph         *depgraph.Graph // .csproj の HintPath から構築した依存グラフ (nil = 未構築)
	graphWarnings []string        // グラフ構築時の警告 (.csproj パース失敗等)

	sel   selectModel // ソリューション選択画面のサブモデル
	build buildModel  // ビルド実行画面のサブモデル

	spinnerFrames []string // スピナーアニメーションのフレーム文字列
	spinnerIdx    int      // 現在表示中のスピナーフレームのインデックス

	Err error // TUI 内部で発生したエラー (スキャン失敗等)。外部から参照可能。
}

// NewModel は TUI モデルを初期化する。
// cmd/root.go から呼ばれ、CLI フラグと設定ファイルからマージされた値が渡される。
//
// 引数:
//   - projectRoot  : プロジェクトルートディレクトリのパス (RelPath の基準)
//   - scanRoots    : スキャン対象サブディレクトリ (projectRoot からの相対、空なら全体)
//   - opts         : ビルドコマンドのオプション (コマンド名、構成、パス)
//   - scanExcludes : スキャン時の追加除外パターン
//   - dllDirMap    : scan root path → コピー先絶対パス (空マップならコピー無効)
//   - maxParallel  : 最大並列ビルド数 (0=無制限)
//   - buildLog     : ビルドツールの stdout/stderr 全文を追記する先 (nil なら書かない)
//
// 初期状態は stateScanning で、Init() により非同期スキャンが開始される。
func NewModel(projectRoot string, scanRoots []string, opts builder.BuildOption, scanExcludes []string, dllDirMap map[string]string, maxParallel int, buildLog io.Writer) Model {
	return Model{
		state:         stateScanning,
		projectRoot:   projectRoot,
		scanRoots:     scanRoots,
		buildOpts:     opts,
		maxParallel:   maxParallel,
		scanExcludes:  scanExcludes,
		dllDirMap:     dllDirMap,
		buildLog:      buildLog,
		spinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// --- Bubble Tea メッセージ型 ---

// scanDoneMsg は非同期 .sln スキャンと依存グラフ構築の完了を通知するメッセージ。
// err が非 nil の場合はスキャン失敗を意味する。
type scanDoneMsg struct {
	solutions     []scanner.Solution // 見つかったソリューション一覧
	graph         *depgraph.Graph    // .csproj HintPath から構築した依存グラフ (構築失敗時は nil)
	graphWarnings []string           // グラフ構築時の警告 (.csproj パース失敗等)
	err           error              // スキャン中に発生したエラー (正常時は nil)
}

// tickMsg はスピナーアニメーションの更新タイミングを通知するメッセージ。
// 80ms 間隔で定期的に送信される。
type tickMsg struct{}

// buildBatchMsg はビルド完了時にログ行と結果をまとめて返すメッセージ。
// 並列実行時は複数の buildBatchMsg が独立して到着するため、
// itemIdx でどのアイテムの結果かを識別する。
type buildBatchMsg struct {
	itemIdx int                // 完了したアイテムの items 内インデックス
	logs    []string           // ビルド中に出力された全ログ行
	result  builder.BuildResult // ビルドの最終結果
}

// tickCmd は 80ms 後に tickMsg を送信する Bubble Tea コマンドを返す。
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
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
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
func (m Model) View() string {
	switch m.state {
	case stateScanning:
		spinner := m.spinnerFrames[m.spinnerIdx]
		return titleStyle.Render("CS Builder") + "\n\n" +
			spinnerStyle.Render(spinner) + " .sln ファイルを探索中...\n"
	case stateSelecting:
		return m.sel.view(m.height)
	case stateBuilding:
		spinner := m.spinnerFrames[m.spinnerIdx]
		return m.build.view(spinner, m.height)
	case stateDone:
		return m.build.view("", m.height)
	}
	return ""
}

// --- キー入力ハンドラ ---

// handleKey はキー入力メッセージを画面状態に応じて振り分ける。
// Ctrl+C はどの画面でもアプリケーションを即座に終了する。
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.state {
	case stateSelecting:
		return m.handleSelectKey(msg)
	case stateDone:
		key := msg.String()
		if key == "enter" || key == "q" {
			return m, tea.Quit
		}
		return m.handleBuildListScrollKey(msg)
	}
	return m, nil
}

// handleBuildListScrollKey はビルド完了画面 (stateDone) で一覧を手動スクロールする (↑↓ jk PgUp/PgDn Home/End)。
func (m Model) handleBuildListScrollKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	itemVP, _ := m.build.layoutHeights(m.height)
	if itemVP <= 0 || len(m.build.items) <= itemVP {
		return m, nil
	}
	key := msg.String()
	switch key {
	case "up", "k":
		m.build.scrollBuildList(-1, itemVP)
	case "down", "j":
		m.build.scrollBuildList(1, itemVP)
	case "pgup":
		m.build.scrollBuildList(-itemVP, itemVP)
	case "pgdown":
		m.build.scrollBuildList(itemVP, itemVP)
	case "home":
		m.build.scrollBuildListHome(itemVP)
	case "end":
		m.build.scrollBuildListEnd(itemVP)
	default:
		return m, nil
	}
	return m, nil
}

// handleSelectKey はソリューション選択画面でのキー入力を処理する。
func (m Model) handleSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.sel.filtering {
		switch key {
		case "esc":
			m.sel = m.sel.clearFilter()
			return m, nil
		case "backspace":
			m.sel = m.sel.deleteFilterChar()
			return m, nil
		case "up":
			m.sel = m.sel.cursorUp()
			return m, nil
		case "down":
			m.sel = m.sel.cursorDown()
			return m, nil
		case "j", "k", "a", "q":
			if msg.Type == tea.KeyRunes {
				for _, r := range msg.Runes {
					m.sel = m.sel.appendFilterChar(r)
				}
			} else if len(key) > 0 {
				m.sel = m.sel.appendFilterChar(rune(key[0]))
			}
			return m, nil
		case " ", "enter":
			// 外側の switch でトグル・確定などを処理する
		default:
			if msg.Type == tea.KeyRunes {
				for _, r := range msg.Runes {
					m.sel = m.sel.appendFilterChar(r)
				}
			}
			return m, nil
		}
	}

	switch key {
	case "up", "k":
		m.sel = m.sel.cursorUp()
	case "down", "j":
		m.sel = m.sel.cursorDown()
	case " ":
		m.sel = m.sel.toggle()
	case "a":
		m.sel = m.sel.toggleAll()
	case "/":
		m.sel = m.sel.startFilter()
	case "enter":
		if !m.sel.hasSelection() {
			return m, nil
		}
		selected := m.sel.selectedSolutions()
		names := make([]string, len(selected))
		for i, s := range selected {
			names[i] = s.RelPath
		}
		slog.Info("build requested", "solutions", names)
		nodes := sortByDependency(m.graph, selected)

		topo := make([]string, 0, len(nodes))
		for _, n := range nodes {
			topo = append(topo, n.Solution.RelPath)
		}
		slog.Info("build order", "topo", topo)
		if m.graph != nil {
			_, dep := m.graph.ScheduleFromSorted(nodes)
			if dep != nil {
				high := make([]string, 0)
				low := make([]string, 0)
				for i, n := range nodes {
					if len(dep[i]) > 0 {
						high = append(high, n.Solution.RelPath)
					} else {
						low = append(low, n.Solution.RelPath)
					}
				}
				slog.Debug("build scheduler priority", "high", high, "low", low)
			}
		}

		m.build = newBuildModel(m.graph, nodes, m.maxParallel)
		for _, w := range m.graphWarnings {
			m.build.appendLog(fmt.Sprintf("[warn] 依存解析: %s", w))
		}
		m.state = stateBuilding
		return m, m.fillBuildSlots()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// --- 非同期コマンド ---

// scanCmd は .sln ファイルの非同期スキャンと依存グラフの構築を実行する。
func (m Model) scanCmd() tea.Cmd {
	projectRoot := m.projectRoot
	scanRoots := m.scanRoots
	excludes := m.scanExcludes
	return func() tea.Msg {
		slog.Info("scan started", "projectRoot", projectRoot, "scanRoots", scanRoots)
		solutions, err := scanner.ScanMultiple(projectRoot, scanRoots, excludes)
		if err != nil {
			slog.Error("scan failed", "error", err)
			return scanDoneMsg{err: err}
		}
		slog.Info("scan completed", "solutions", len(solutions))
		graph, warnings := depgraph.Build(solutions)
		for _, w := range warnings {
			slog.Warn("dependency graph warning", "detail", w)
		}

		edges := graph.InternalEdges()
		edgeCount := 0
		for _, deps := range edges {
			edgeCount += len(deps)
		}
		slog.Info("dependency graph built", "nodes", len(graph.Nodes()), "edges", edgeCount)
		for name, deps := range edges {
			slog.Debug("node dependencies", "assembly", name, "dependsOn", deps)
		}

		return scanDoneMsg{
			solutions:     solutions,
			graph:         graph,
			graphWarnings: warnings,
		}
	}
}

// handleScanDone はスキャン完了メッセージを処理する。
func (m Model) handleScanDone(msg scanDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.Err = msg.err
		return m, tea.Quit
	}
	if len(msg.solutions) == 0 {
		m.state = stateDone
		m.build = newBuildModel(nil, nil, m.maxParallel)
		return m, nil
	}
	m.graph = msg.graph
	m.graphWarnings = msg.graphWarnings
	m.sel = newSelectModel(msg.solutions)
	m.state = stateSelecting
	return m, nil
}

// fillBuildSlots は準備完了かつ優先度に従って、並列枠に収まる分のビルドを起動する。
// 起動したビルド Cmd を tea.Batch でまとめて返す。
func (m *Model) fillBuildSlots() tea.Cmd {
	var cmds []tea.Cmd
	for _, idx := range m.build.pickNextToStart() {
		m.build.items[idx].status = statusBuilding
		m.build.activeCount++
		cmds = append(cmds, m.runBuildForItem(idx))
	}
	m.build.refreshDisplayOrder()
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// runBuildForItem は指定インデックスのアイテムに対してビルドを実行する
// Bubble Tea コマンドを返す。
func (m Model) runBuildForItem(idx int) tea.Cmd {
	item := m.build.items[idx]
	opts := m.buildOpts
	return func() tea.Msg {
		logCh := make(chan string, 64)
		doneCh := make(chan builder.BuildResult, 1)

		go func() {
			result := builder.Build(context.Background(), item.solution.AbsPath, opts, logCh)
			close(logCh)
			doneCh <- result
		}()

		var logLines []string
		for line := range logCh {
			logLines = append(logLines, line)
		}

		result := <-doneCh
		return buildBatchMsg{itemIdx: idx, logs: logLines, result: result}
	}
}

// handleBuildBatch はビルド完了メッセージを処理する。
//
// 処理の流れ:
//  1. ログ行を buildModel に追加する
//  2. 完了したアイテムの結果を記録する (completeItem)
//  3. ビルド成功時に成果物を共有 DLL ディレクトリにコピーする
//  4. 全アイテム完了なら Done 画面に遷移
//  5. 未完了なら fillBuildSlots() で準備完了分を優先度付きで起動
func (m Model) handleBuildBatch(msg buildBatchMsg) (Model, tea.Cmd) {
	for _, line := range msg.logs {
		m.build.appendLog(line)
	}
	m.build.completeItem(msg.itemIdx, msg.result)

	if m.buildLog != nil {
		fmt.Fprintf(m.buildLog, "==== %s success=%v duration=%s ====\n",
			msg.result.Solution, msg.result.Success, msg.result.Duration.String())
		fmt.Fprint(m.buildLog, msg.result.Output)
		if len(msg.result.Output) > 0 && !strings.HasSuffix(msg.result.Output, "\n") {
			fmt.Fprintln(m.buildLog)
		}
		fmt.Fprintln(m.buildLog)
	}

	slog.Info("build completed",
		"solution", msg.result.Solution,
		"success", msg.result.Success,
		"duration", msg.result.Duration.String(),
	)

	if msg.result.Success && len(m.dllDirMap) > 0 {
		if dllDir, scanRootAbs, ok := m.resolveDllDir(msg.result.Solution); ok {
			if err := artifact.CopyArtifact(msg.result.Solution, m.buildOpts.Configuration, dllDir, scanRootAbs); err != nil {
				slog.Warn("artifact copy failed", "solution", msg.result.Solution, "error", err)
				m.build.appendLog(fmt.Sprintf("[warn] DLL コピー失敗: %v", err))
			} else {
				slog.Debug("artifact copied", "solution", msg.result.Solution, "dllDir", dllDir)
			}
		}
	}

	if m.build.done {
		m.state = stateDone
		slog.Info("all builds done")
		return m, nil
	}
	return m, m.fillBuildSlots()
}

// sortByDependency は依存グラフに基づいてビルド順をソートする。
// Level 情報付きの []*depgraph.Node を返す。
// グラフが nil またはソートに失敗した場合は全ノード Level=0 でフォールバック。
func sortByDependency(g *depgraph.Graph, selected []scanner.Solution) []*depgraph.Node {
	if g != nil {
		sorted, err := g.Sort(selected)
		if err == nil {
			return sorted
		}
	}
	nodes := make([]*depgraph.Node, len(selected))
	for i, s := range selected {
		nodes[i] = &depgraph.Node{Solution: s, Level: 0}
	}
	return nodes
}

// resolveDllDir は .sln の絶対パスから所属する scan root を判定し、
// コピー先ディレクトリと scan root の絶対パスを返す。該当なしの場合は ok=false。
func (m Model) resolveDllDir(slnAbsPath string) (dllDir string, scanRootAbs string, ok bool) {
	rel, err := filepath.Rel(m.projectRoot, slnAbsPath)
	if err != nil {
		return "", "", false
	}
	relSlash := filepath.ToSlash(rel)
	for root, dir := range m.dllDirMap {
		prefix := filepath.ToSlash(root) + "/"
		if strings.HasPrefix(relSlash, prefix) {
			return dir, filepath.Join(m.projectRoot, root), true
		}
	}
	return "", "", false
}

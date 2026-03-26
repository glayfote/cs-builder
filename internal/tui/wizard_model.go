package tui

// このファイルは Bubble Tea の Model 実装（wizardModel）と画面遷移・ビルド実行コマンドを定義する。
// フェーズ順は Empty → Configuration → Tenant → SlnPick → Confirm → BuildRun → Summary。

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"builder/cs-builder/internal/buildorder"
	"builder/cs-builder/internal/config"
	"builder/cs-builder/internal/dotnetpath"
	"builder/cs-builder/internal/scan"
)

// wizardPhase はウィザードの画面状態（どの質問を出しているか）を表す。
type wizardPhase int

const (
	wizardPhaseEmpty wizardPhase = iota
	wizardPhaseNoMatch
	wizardPhaseConfiguration
	wizardPhaseTenant
	wizardPhaseSlnPick
	wizardPhaseConfirm
	wizardPhaseBuildRun
	wizardPhaseSummary
)

// buildDoneMsg は tea.ExecProcess で起動した dotnet build が終了したときにバブルに送るメッセージ。
type buildDoneMsg struct {
	path string
	err  error
}

// wizardModel は tea.Model を実装する。Update / View がメインループから呼ばれる。
type wizardModel struct {
	phase  wizardPhase
	cursor int
	// flash は 1 フレームだけ表示するエラー・注意文（バリデーション失敗や Resolve 失敗など）。
	flash string

	cfg             *config.Config
	allSolutions    []scan.Solution  // scan.FindSolutions の生結果（テナントフィルタ前）
	filtered        []scan.Solution  // 選択テナントに一致した .sln のみ
	configuration   string           // Debug / Release
	tenant          string           // フィルタに使ったテナント識別子（TenantAll 可）
	pickEntries     []pickEntry      // SlnPick に並べる行（1 行 1 .sln）
	selected        map[int]struct{} // pickEntries の添字 → チェック済み
	targetPaths     []string         // 確定した .sln の絶対パス
	orderedProjects []string         // buildorder.Resolve 後の .csproj ビルド列
	buildIdx        int              // orderedProjects 内の現在位置（buildDoneMsg でインクリメント）
	buildFailures   []BuildFailure
	finalResult     WizardResult // Summary で Quit 前に populateResult で埋める
	aborted         bool         // q / Esc による意図的な中断
	dotnetExec      string       // 確認画面通過時に dotnetpath.Resolve で一度だけ設定
}

// newWizardModel は初期フェーズをスキャン件数に応じて Empty または Configuration に設定する。
func newWizardModel(all []scan.Solution, cfg *config.Config) *wizardModel {
	m := &wizardModel{allSolutions: all, cfg: cfg}
	if len(all) == 0 {
		m.phase = wizardPhaseEmpty
	} else {
		m.phase = wizardPhaseConfiguration
	}
	return m
}

// Init は初回のみ呼ばれる。非同期コマンドは不要なので nil。
func (m *wizardModel) Init() tea.Cmd {
	return nil
}

// title は現在フェーズの見出し文言を返す。
func (m *wizardModel) title() string {
	switch m.phase {
	case wizardPhaseEmpty:
		return "スキャン結果"
	case wizardPhaseNoMatch:
		return "テナント"
	case wizardPhaseConfiguration:
		return "ビルド構成を選んでください"
	case wizardPhaseTenant:
		return "テナントを選んでください"
	case wizardPhaseSlnPick:
		return "ビルドする .sln を選んでください（Space で複数チェック）"
	case wizardPhaseConfirm:
		return "確認"
	case wizardPhaseBuildRun:
		return "ビルド実行中"
	case wizardPhaseSummary:
		return "完了"
	default:
		return ""
	}
}

// currentChoices はリスト選択型フェーズで表示する選択肢のスライスを返す。
func (m *wizardModel) currentChoices() []string {
	switch m.phase {
	case wizardPhaseConfiguration:
		return []string{ConfigDebug, ConfigRelease}
	case wizardPhaseTenant:
		return tenantChoices
	default:
		return nil
	}
}

func (m *wizardModel) clearFlash() {
	m.flash = ""
}

// Update はキー入力と非同期ビルド完了を処理する。buildDoneMsg 受信時に次の dotnet をキューする。
func (m *wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.clearFlash()

	switch msg := msg.(type) {
	case buildDoneMsg:
		if msg.err != nil {
			m.buildFailures = append(m.buildFailures, BuildFailure{Path: msg.path, Err: msg.err})
		}
		m.buildIdx++
		if m.buildIdx >= len(m.orderedProjects) {
			m.phase = wizardPhaseSummary
			return m, nil
		}
		return m, m.runCurrentBuildCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			if m.phase == wizardPhaseSummary {
				m.populateResult()
				return m, tea.Quit
			}
			m.aborted = true
			return m, tea.Quit

		case "b", "left":
			m.goBack()
			return m, nil

		case "up", "k":
			if m.phase == wizardPhaseSlnPick && len(m.pickEntries) > 0 {
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			}
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.phase == wizardPhaseSlnPick && len(m.pickEntries) > 0 {
				if m.cursor < len(m.pickEntries)-1 {
					m.cursor++
				}
				return m, nil
			}
			choices := m.currentChoices()
			if m.cursor < len(choices)-1 {
				m.cursor++
			}
			return m, nil

		case " ":
			if m.phase == wizardPhaseSlnPick {
				m.toggleSlnPickSelection()
				return m, nil
			}

		case "enter":
			return m.handleEnter()
		}
	}
	return m, nil
}

// goBack は b / ← 用の親フェーズへの遷移。BuildRun / Summary からは戻れない。
func (m *wizardModel) goBack() {
	switch m.phase {
	case wizardPhaseEmpty:
	case wizardPhaseNoMatch:
		m.phase = wizardPhaseTenant
		m.cursor = indexInChoices(m.tenant, tenantChoices)
	case wizardPhaseConfiguration:
	case wizardPhaseTenant:
		m.phase = wizardPhaseConfiguration
		m.tenant = ""
		m.filtered = nil
		m.cursor = indexInChoices(m.configuration, []string{ConfigDebug, ConfigRelease})
	case wizardPhaseSlnPick:
		m.phase = wizardPhaseTenant
		m.pickEntries = nil
		m.selected = nil
		m.cursor = indexInChoices(m.tenant, tenantChoices)
	case wizardPhaseConfirm:
		m.phase = wizardPhaseSlnPick
		m.cursor = 0
	case wizardPhaseBuildRun:
	case wizardPhaseSummary:
	}
}

// toggleSlnPickSelection は SlnPick フェーズで Space によりチェックを付け外しする（複数選択可）。
func (m *wizardModel) toggleSlnPickSelection() {
	if m.phase != wizardPhaseSlnPick || len(m.pickEntries) == 0 {
		return
	}
	i := m.cursor
	if _, ok := m.selected[i]; ok {
		delete(m.selected, i)
		return
	}
	if m.selected == nil {
		m.selected = make(map[int]struct{})
	}
	m.selected[i] = struct{}{}
}

// handleEnter は Enter キー確定時の遷移。Confirm では buildorder.Resolve を呼びから BuildRun へ入る。
func (m *wizardModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.phase {
	case wizardPhaseEmpty:
		m.aborted = true
		return m, tea.Quit

	case wizardPhaseNoMatch:
		return m, nil

	case wizardPhaseConfiguration:
		ch := m.currentChoices()
		if len(ch) == 0 {
			return m, nil
		}
		m.configuration = ch[m.cursor]
		m.phase = wizardPhaseTenant
		m.cursor = 0
		return m, nil

	case wizardPhaseTenant:
		ch := m.currentChoices()
		m.tenant = ch[m.cursor]
		m.filtered = filterByTenant(m.allSolutions, m.tenant)
		if len(m.filtered) == 0 {
			m.phase = wizardPhaseNoMatch
			m.cursor = 0
			return m, nil
		}
		m.pickEntries = buildSlnPickEntries(m.filtered)
		m.selected = make(map[int]struct{})
		m.cursor = 0
		m.phase = wizardPhaseSlnPick
		if len(m.pickEntries) == 0 {
			m.flash = "候補がありません。b で戻ってください。"
		}
		return m, nil

	case wizardPhaseSlnPick:
		if len(m.selected) < 1 {
			m.flash = "1 件以上チェックを入れてください。"
			return m, nil
		}
		m.targetPaths = pathsFromPickSelection(m.pickEntries, m.selected)
		if len(m.targetPaths) == 0 {
			m.flash = "対象の .sln がありません。"
			return m, nil
		}
		m.phase = wizardPhaseConfirm
		m.cursor = 0
		return m, nil

	case wizardPhaseConfirm:
		order, err := buildorder.Resolve(m.cfg, m.targetPaths)
		if err != nil {
			m.flash = "依存グラフ: " + err.Error()
			return m, nil
		}
		dot, err := dotnetpath.Resolve()
		if err != nil {
			m.flash = err.Error()
			return m, nil
		}
		m.dotnetExec = dot
		m.orderedProjects = order
		m.phase = wizardPhaseBuildRun
		m.buildIdx = 0
		m.buildFailures = nil
		return m, m.runCurrentBuildCmd()

	case wizardPhaseBuildRun:
		return m, nil

	case wizardPhaseSummary:
		m.populateResult()
		return m, tea.Quit
	}
	return m, nil
}

// runCurrentBuildCmd は orderedProjects[buildIdx] に対して dotnet build を子プロセスで起動する。
func (m *wizardModel) runCurrentBuildCmd() tea.Cmd {
	if m.buildIdx >= len(m.orderedProjects) {
		return nil
	}
	idx := m.buildIdx
	path := m.orderedProjects[idx]
	exe := m.dotnetExec
	if exe == "" {
		var err error
		exe, err = dotnetpath.Resolve()
		if err != nil {
			return func() tea.Msg {
				return buildDoneMsg{path: path, err: err}
			}
		}
		m.dotnetExec = exe
	}
	c := exec.Command(exe, "build", path, "-c", m.configuration)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return buildDoneMsg{path: path, err: err}
	})
}

// populateResult は Summary 終了時に finalResult へ画面状態をコピーする。
func (m *wizardModel) populateResult() {
	m.finalResult = WizardResult{
		Configuration:       m.configuration,
		Tenant:              m.tenant,
		SolutionPaths:       append([]string(nil), m.targetPaths...),
		OrderedProjectPaths: append([]string(nil), m.orderedProjects...),
		BuildFailures:       append([]BuildFailure(nil), m.buildFailures...),
	}
}

// View は現在フェーズの画面文字列を組み立てる（リップグロス等は未使用のプレーンテキスト）。
func (m *wizardModel) View() string {
	var b strings.Builder
	if m.flash != "" {
		b.WriteString(m.flash)
		b.WriteString("\n\n")
	}
	b.WriteString(m.title())
	b.WriteString("\n\n")

	switch m.phase {
	case wizardPhaseEmpty:
		b.WriteString("スキャン結果が 0 件です。cs-builder.yaml の scan_roots 等を確認してください。\n\nq で終了\n")

	case wizardPhaseNoMatch:
		b.WriteString("このテナントに該当する .sln がありません。\n\nb / ← でテナント選択に戻る · q で終了\n")

	case wizardPhaseConfiguration, wizardPhaseTenant:
		for i, name := range m.currentChoices() {
			cur := "  "
			if i == m.cursor {
				cur = "> "
			}
			b.WriteString(cur)
			b.WriteString(name)
			b.WriteByte('\n')
		}
		b.WriteString(m.footerListPhase())

	case wizardPhaseSlnPick:
		for i, e := range m.pickEntries {
			mark := " "
			if _, ok := m.selected[i]; ok {
				mark = "x"
			}
			cur := "  "
			if i == m.cursor {
				cur = "> "
			}
			// 1 行 1 エントリ: .sln の絶対パスのみ表示する。
			fmt.Fprintf(&b, "%s[%s] %s\n", cur, mark, e.Label)
		}
		b.WriteString("\nSpace チェック · Enter 確定 · b / ← 戻る · q 終了\n")
		if m.configuration != "" {
			b.WriteString("\n構成: ")
			b.WriteString(m.configuration)
			b.WriteByte('\n')
		}

	case wizardPhaseConfirm:
		b.WriteString("構成: ")
		b.WriteString(m.configuration)
		b.WriteString("\nテナント: ")
		b.WriteString(m.tenant)
		fmt.Fprintf(&b, "\n\n選択した .sln: %d 件\n", len(m.targetPaths))
		b.WriteString(joinPathsPreview(m.targetPaths, 8))
		b.WriteString("\n\nEnter で依存順に dotnet build（.csproj）開始 · b / ← で戻る · q 終了\n")

	case wizardPhaseBuildRun:
		fmt.Fprintf(&b, "プロジェクト (%d / %d)\n\n", m.buildIdx+1, len(m.orderedProjects))
		if m.buildIdx < len(m.orderedProjects) {
			b.WriteString(m.orderedProjects[m.buildIdx])
			b.WriteByte('\n')
		}

	case wizardPhaseSummary:
		ok := len(m.orderedProjects) - len(m.buildFailures)
		fmt.Fprintf(&b, "成功: %d  失敗: %d\n\n", ok, len(m.buildFailures))
		for _, f := range m.buildFailures {
			b.WriteString(f.Path)
			b.WriteString("\n  ")
			b.WriteString(f.Err.Error())
			b.WriteString("\n\n")
		}
		b.WriteString("Enter / q で終了\n")
	}

	return b.String()
}

// footerListPhase はリスト系フェーズのフッター（キーバインド案内）を返す。
func (m *wizardModel) footerListPhase() string {
	var parts []string
	parts = append(parts, "↑/↓ または j/k · Enter で確定 · q / Esc で終了")
	switch m.phase {
	case wizardPhaseTenant:
		parts = append(parts, "b / ← で構成に戻る")
	case wizardPhaseSlnPick:
		parts = append(parts, "b / ← でテナントに戻る")
	}
	if m.phase == wizardPhaseTenant && m.configuration != "" {
		return "\n" + strings.Join(parts, " · ") + "\n\n現在の構成: " + m.configuration + "\n"
	}
	return "\n" + strings.Join(parts, " · ") + "\n"
}

// indexInChoices は value に一致する選択肢の添字を返す。無ければ 0。
func indexInChoices(value string, choices []string) int {
	for i, v := range choices {
		if v == value {
			return i
		}
	}
	return 0
}

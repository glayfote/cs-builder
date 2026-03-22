package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"builder/cs-builder/internal/scan"
)

type wizardPhase int

const (
	wizardPhaseEmpty wizardPhase = iota
	wizardPhaseNoMatch
	wizardPhaseConfiguration
	wizardPhaseTenant
	wizardPhasePackageMode
	wizardPhasePickKind
	wizardPhasePickList
	wizardPhaseConfirm
	wizardPhaseBuildRun
	wizardPhaseSummary
)

type buildDoneMsg struct {
	path string
	err  error
}

type wizardModel struct {
	phase  wizardPhase
	cursor int
	flash  string

	allSolutions    []scan.Solution
	filtered        []scan.Solution
	configuration   string
	tenant          string
	singlePackage   bool
	pickIndividual  bool
	pickEntries     []pickEntry
	selected        map[int]struct{}
	targetPaths     []string
	buildIdx        int
	buildFailures   []BuildFailure
	finalResult     WizardResult
	aborted         bool
}

func newWizardModel(all []scan.Solution) *wizardModel {
	m := &wizardModel{allSolutions: all}
	if len(all) == 0 {
		m.phase = wizardPhaseEmpty
	} else {
		m.phase = wizardPhaseConfiguration
	}
	return m
}

func (m *wizardModel) Init() tea.Cmd {
	return nil
}

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
	case wizardPhasePackageMode:
		return "パッケージの選び方"
	case wizardPhasePickKind:
		return "対象の指定方法"
	case wizardPhasePickList:
		if m.pickIndividual {
			return "パッケージを選んでください（Space でチェック）"
		}
		return "フォルダ（スコープ）を選んでください（Space でチェック）"
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

func (m *wizardModel) currentChoices() []string {
	switch m.phase {
	case wizardPhaseConfiguration:
		return []string{ConfigDebug, ConfigRelease}
	case wizardPhaseTenant:
		return tenantChoices
	case wizardPhasePackageMode:
		return packageModeChoices
	case wizardPhasePickKind:
		return pickKindChoices
	default:
		return nil
	}
}

func (m *wizardModel) clearFlash() {
	m.flash = ""
}

func (m *wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.clearFlash()

	switch msg := msg.(type) {
	case buildDoneMsg:
		if msg.err != nil {
			m.buildFailures = append(m.buildFailures, BuildFailure{Path: msg.path, Err: msg.err})
		}
		m.buildIdx++
		if m.buildIdx >= len(m.targetPaths) {
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
			if m.phase == wizardPhasePickList && len(m.pickEntries) > 0 {
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
			if m.phase == wizardPhasePickList && len(m.pickEntries) > 0 {
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
			if m.phase == wizardPhasePickList {
				m.togglePickSelection()
				return m, nil
			}

		case "enter":
			return m.handleEnter()
		}
	}
	return m, nil
}

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
	case wizardPhasePackageMode:
		m.phase = wizardPhaseTenant
		m.cursor = indexInChoices(m.tenant, tenantChoices)
	case wizardPhasePickKind:
		m.phase = wizardPhasePackageMode
		m.cursor = 0
		if m.singlePackage {
			m.cursor = 0
		} else {
			m.cursor = 1
		}
	case wizardPhasePickList:
		m.phase = wizardPhasePickKind
		m.pickEntries = nil
		m.selected = nil
		m.cursor = 0
		if m.pickIndividual {
			m.cursor = 0
		} else {
			m.cursor = 1
		}
	case wizardPhaseConfirm:
		m.phase = wizardPhasePickList
		m.cursor = 0
	case wizardPhaseBuildRun:
	case wizardPhaseSummary:
	}
}

func (m *wizardModel) togglePickSelection() {
	if m.phase != wizardPhasePickList || len(m.pickEntries) == 0 {
		return
	}
	i := m.cursor
	if m.singlePackage {
		if _, ok := m.selected[i]; ok {
			delete(m.selected, i)
			return
		}
		m.selected = map[int]struct{}{i: {}}
		return
	}
	if _, ok := m.selected[i]; ok {
		delete(m.selected, i)
	} else {
		if m.selected == nil {
			m.selected = make(map[int]struct{})
		}
		m.selected[i] = struct{}{}
	}
}

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
		m.phase = wizardPhasePackageMode
		m.cursor = 0
		return m, nil

	case wizardPhasePackageMode:
		ch := m.currentChoices()
		m.singlePackage = (ch[m.cursor] == packageModeChoices[0])
		m.phase = wizardPhasePickKind
		m.cursor = 0
		return m, nil

	case wizardPhasePickKind:
		ch := m.currentChoices()
		m.pickIndividual = (ch[m.cursor] == pickKindChoices[0])
		if m.pickIndividual {
			m.pickEntries = buildPackagePickEntries(m.filtered)
		} else {
			m.pickEntries = buildFolderPickEntries(m.filtered)
		}
		m.selected = make(map[int]struct{})
		m.cursor = 0
		m.phase = wizardPhasePickList
		if len(m.pickEntries) == 0 {
			m.flash = "候補がありません。b で戻ってください。"
		}
		return m, nil

	case wizardPhasePickList:
		n := len(m.selected)
		if m.singlePackage && n != 1 {
			m.flash = "単一パッケージでは 1 件にチェックを入れてください。"
			return m, nil
		}
		if !m.singlePackage && n < 1 {
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

func (m *wizardModel) runCurrentBuildCmd() tea.Cmd {
	if m.buildIdx >= len(m.targetPaths) {
		return nil
	}
	idx := m.buildIdx
	path := m.targetPaths[idx]
	c := exec.Command("dotnet", "build", path, "-c", m.configuration)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return buildDoneMsg{path: path, err: err}
	})
}

func (m *wizardModel) populateResult() {
	m.finalResult = WizardResult{
		Configuration:  m.configuration,
		Tenant:         m.tenant,
		SinglePackage:  m.singlePackage,
		PickIndividual: m.pickIndividual,
		SolutionPaths:  append([]string(nil), m.targetPaths...),
		BuildFailures:  append([]BuildFailure(nil), m.buildFailures...),
	}
}

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

	case wizardPhaseConfiguration, wizardPhaseTenant, wizardPhasePackageMode, wizardPhasePickKind:
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

	case wizardPhasePickList:
		for i, e := range m.pickEntries {
			mark := " "
			if _, ok := m.selected[i]; ok {
				mark = "x"
			}
			cur := "  "
			if i == m.cursor {
				cur = "> "
			}
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
		b.WriteString("\nパッケージ: ")
		if m.singlePackage {
			b.WriteString("単一")
		} else {
			b.WriteString("複数")
		}
		b.WriteString("\n指定: ")
		if m.pickIndividual {
			b.WriteString("個別パッケージ")
		} else {
			b.WriteString("フォルダ配下すべて")
		}
		fmt.Fprintf(&b, "\n\n対象 %d 件:\n", len(m.targetPaths))
		b.WriteString(joinPathsPreview(m.targetPaths, 12))
		b.WriteString("\n\nEnter で dotnet build 開始 · b / ← で戻る · q 終了\n")

	case wizardPhaseBuildRun:
		fmt.Fprintf(&b, "(%d / %d)\n\n", m.buildIdx+1, len(m.targetPaths))
		if m.buildIdx < len(m.targetPaths) {
			b.WriteString(m.targetPaths[m.buildIdx])
			b.WriteByte('\n')
		}

	case wizardPhaseSummary:
		ok := len(m.targetPaths) - len(m.buildFailures)
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

func (m *wizardModel) footerListPhase() string {
	var parts []string
	parts = append(parts, "↑/↓ または j/k · Enter で確定 · q / Esc で終了")
	switch m.phase {
	case wizardPhaseTenant:
		parts = append(parts, "b / ← で構成に戻る")
	case wizardPhasePackageMode:
		parts = append(parts, "b / ← でテナントに戻る")
	case wizardPhasePickKind:
		parts = append(parts, "b / ← でパッケージ数に戻る")
	}
	if m.phase == wizardPhaseTenant && m.configuration != "" {
		return "\n" + strings.Join(parts, " · ") + "\n\n現在の構成: " + m.configuration + "\n"
	}
	return "\n" + strings.Join(parts, " · ") + "\n"
}

func indexInChoices(value string, choices []string) int {
	for i, v := range choices {
		if v == value {
			return i
		}
	}
	return 0
}

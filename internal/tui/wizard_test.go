package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"builder/cs-builder/internal/config"
	"builder/cs-builder/internal/scan"
)

func testWizardCfg() *config.Config {
	c := &config.Config{Version: 1, ProjectRoot: ".", ScanRoots: []string{"x"}}
	c.ApplyDefaults()
	return c
}

func sampleSolutions() []scan.Solution {
	return []scan.Solution{
		{Path: "/r/2_if/logger/A.sln", ScanRoot: "2_if", PackageDir: "logger", Tenant: "tenant1"},
		{Path: "/r/2_if/sql/tenant2/B.sln", ScanRoot: "2_if", PackageDir: "sql", Tenant: "tenant2"},
		{Path: "/r/3_driver/x/C.sln", ScanRoot: "3_driver", PackageDir: "x", Tenant: ""},
	}
}

func TestNewWizardEmpty(t *testing.T) {
	m := newWizardModel(nil, testWizardCfg())
	if m.phase != wizardPhaseEmpty {
		t.Fatalf("phase = %v, want Empty", m.phase)
	}
}

func TestWizard_ConfigThenTenantAll(t *testing.T) {
	var m tea.Model = newWizardModel(sampleSolutions(), testWizardCfg())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w := m.(*wizardModel)
	if w.phase != wizardPhaseTenant {
		t.Fatalf("phase = %v, want Tenant", w.phase)
	}
	// tenant all is last index (4 downs from tenant1)
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w = m.(*wizardModel)
	if w.phase != wizardPhaseSlnPick {
		t.Fatalf("phase = %v, want SlnPick", w.phase)
	}
	if len(w.filtered) != len(sampleSolutions()) {
		t.Fatalf("filtered len = %d", len(w.filtered))
	}
}

func TestWizard_TenantFilterNoMatch(t *testing.T) {
	var m tea.Model = newWizardModel(sampleSolutions(), testWizardCfg())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// tenant1 is first choice — has matches
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	w := m.(*wizardModel)
	if w.phase != wizardPhaseSlnPick {
		t.Fatalf("phase = %v, want SlnPick", w.phase)
	}
	if len(w.filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1", len(w.filtered))
	}
}

func TestWizard_BackToConfiguration(t *testing.T) {
	var m tea.Model = newWizardModel(sampleSolutions(), testWizardCfg())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	w := m.(*wizardModel)
	if w.phase != wizardPhaseConfiguration {
		t.Fatalf("phase = %v", w.phase)
	}
}

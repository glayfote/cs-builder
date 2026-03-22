package tui

import (
	"testing"
)

func TestFilterByTenantAll(t *testing.T) {
	all := sampleSolutions()
	out := filterByTenant(all, TenantAll)
	if len(out) != len(all) {
		t.Fatalf("len = %d", len(out))
	}
}

func TestFilterByTenant1(t *testing.T) {
	out := filterByTenant(sampleSolutions(), Tenant1)
	if len(out) != 1 || out[0].Tenant != Tenant1 {
		t.Fatalf("got %+v", out)
	}
}

func TestBuildPackagePickEntries(t *testing.T) {
	entries := buildPackagePickEntries(sampleSolutions())
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
}

func TestBuildFolderPickEntries(t *testing.T) {
	entries := buildFolderPickEntries(sampleSolutions())
	if len(entries) < 2 {
		t.Fatalf("len = %d", len(entries))
	}
}

func TestPathsFromPickSelection(t *testing.T) {
	entries := buildPackagePickEntries(sampleSolutions())
	sel := map[int]struct{}{0: {}, 1: {}}
	paths := pathsFromPickSelection(entries, sel)
	if len(paths) != 2 {
		t.Fatalf("paths = %v", paths)
	}
}

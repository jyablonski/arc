package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncRules_CreatesMissing(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	if err := os.MkdirAll(filepath.Dir(paths.RulesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RulesFile, []byte("shared rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := m.SyncRules(); err != nil {
		t.Fatalf("SyncRules: %v", err)
	}
	for _, p := range providers {
		if p.RulesFile == "" {
			continue
		}
		info, err := os.Lstat(p.RulesFile)
		if err != nil {
			t.Errorf("%s: rules file not created: %v", p.Name, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s: rules file is not a symlink", p.Name)
		}
	}
}

func TestSyncRules_RefusesRealFile(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	if err := os.MkdirAll(filepath.Dir(paths.RulesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RulesFile, []byte("shared\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(providers[0].RulesFile[:len(providers[0].RulesFile)-len("/CLAUDE.md")], 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(providers[0].RulesFile, []byte("my rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	conflicts, err := m.SyncRules()
	if err != nil {
		t.Fatalf("SyncRules: %v", err)
	}
	if conflicts != 1 {
		t.Errorf("expected 1 conflict, got %d", conflicts)
	}
	data, _ := os.ReadFile(providers[0].RulesFile)
	if string(data) != "my rules\n" {
		t.Errorf("user content altered: %q", data)
	}
}

func TestSyncRules_CanonicalMissing(t *testing.T) {
	m, _, _ := newTestManager(t, false)
	_, err := m.SyncRules()
	if err == nil {
		t.Fatalf("expected error when canonical missing")
	}
}

func TestSyncRules_FixesStaleSymlink(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	if err := os.MkdirAll(filepath.Dir(paths.RulesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RulesFile, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(providers[0].RulesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	oldTarget := filepath.Join(t.TempDir(), "old.md")
	_ = os.WriteFile(oldTarget, []byte("old\n"), 0o644)
	mustSymlink(t, oldTarget, providers[0].RulesFile)

	if _, err := m.SyncRules(); err != nil {
		t.Fatalf("SyncRules: %v", err)
	}
	target := readlink(t, providers[0].RulesFile)
	if target != paths.RulesFile {
		t.Errorf("stale symlink not updated: %q", target)
	}
}

func TestStatusRules(t *testing.T) {
	m, paths, providers := newTestManager(t, false)
	if err := os.MkdirAll(filepath.Dir(paths.RulesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.RulesFile, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(providers[0].RulesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, paths.RulesFile, providers[0].RulesFile)

	res := m.StatusRules()
	if res.Canonical != paths.RulesFile {
		t.Errorf("Canonical: %s", res.Canonical)
	}
	statuses := map[string]Status{}
	for _, p := range res.Providers {
		statuses[p.Provider] = p.Status
	}
	if statuses["claude"] != StatusOK {
		t.Errorf("claude: got %q, want ok", statuses["claude"])
	}
	if statuses["codex"] != StatusMissing {
		t.Errorf("codex: got %q, want missing", statuses["codex"])
	}
	if _, ok := statuses["cursor"]; ok {
		t.Errorf("cursor should have no entry (no RulesFile)")
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndLoadRules(t *testing.T) {
	dir := t.TempDir()
	r, err := createRule(dir, " Use httpx ", "Every HTTP call goes through httpx.", "Because we say so.")
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "OBS-001" {
		t.Errorf("first id = %q, want OBS-001", r.ID)
	}
	os.Mkdir(filepath.Join(dir, "OBS-007"), 0o755) // a gap, and no rule.md
	if r2, _ := createRule(dir, "second", "", ""); r2.ID != "OBS-008" {
		t.Errorf("next id = %q, want OBS-008", r2.ID)
	}
	os.RemoveAll(filepath.Join(dir, "OBS-008"))
	for _, name := range []string{"rule.md", "explain.md"} {
		if _, err := os.Stat(filepath.Join(r.Dir, name)); err != nil {
			t.Errorf("%s missing: %v", name, err)
		}
	}
	// A directory without rule.md is not a rule.
	os.Mkdir(filepath.Join(dir, "junk"), 0o755)

	rules, err := loadRules(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}
	got := rules[0]
	if got.ID != r.ID || got.Title != "Use httpx" || got.Summary() != "Every HTTP call goes through httpx." {
		t.Errorf("loaded %+v", got)
	}
	if err := deleteRule(got); err != nil {
		t.Fatal(err)
	}
	if rules, _ := loadRules(dir); len(rules) != 0 {
		t.Errorf("rule still there after delete")
	}
}

func TestCreateRuleNeedsTitle(t *testing.T) {
	if _, err := createRule(t.TempDir(), "  ", "", ""); err == nil {
		t.Error("expected an error for an empty title")
	}
}

func TestLoadRulesMissingDir(t *testing.T) {
	if _, err := loadRules(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

func TestInitRepo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), rulesDirName)
	created, err := initRepo(dir)
	if err != nil || !created {
		t.Fatalf("first init: created=%v err=%v", created, err)
	}
	if cfg := loadConfig(dir); cfg != defaultConfig() {
		t.Errorf("template config %+v differs from defaults %+v", cfg, defaultConfig())
	}
	os.WriteFile(filepath.Join(dir, "config"), []byte("model = x\n"), 0o644)
	if created, _ := initRepo(dir); created {
		t.Error("second init overwrote the config")
	}
}

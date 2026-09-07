package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	if got := loadConfig(dir); got != defaultConfig() {
		t.Errorf("no file: got %+v, want defaults", got)
	}
	content := `# observatory settings
model = "claude-sonnet-5"   # trailing comment
max_tokens = 50000
max_output_tokens = nonsense
per_rule = true
`
	os.WriteFile(filepath.Join(dir, "config"), []byte(content), 0o644)
	got := loadConfig(dir)
	want := Config{Provider: "anthropic", Model: "claude-sonnet-5", BaseURL: "https://api.anthropic.com", APIKeyEnv: "ANTHROPIC_API_KEY", MaxTokens: 50000, MaxOutputTokens: defaultConfig().MaxOutputTokens, PerRule: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	t.Setenv("OBSERVATORY_PROVIDER", "openai")
	t.Setenv("OBSERVATORY_MODEL", "llama3")
	got = loadConfig(dir)
	if got.Provider != "openai" || got.Model != "llama3" || got.BaseURL != "https://api.openai.com/v1" || got.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("env override: got %+v", got)
	}
	if _, err := newProvider(Config{Provider: "gemini"}); err == nil {
		t.Error("unknown provider did not fail")
	}
}

func TestEditorCommand(t *testing.T) {
	t.Setenv("OBSERVATORY_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "vim -p")
	if got := editorCommand(); len(got) != 2 || got[0] != "vim" || got[1] != "-p" {
		t.Errorf("EDITOR: got %v", got)
	}
	t.Setenv("OBSERVATORY_EDITOR", "code -n")
	if got := editorCommand(); got[0] != "code" {
		t.Errorf("OBSERVATORY_EDITOR did not win: got %v", got)
	}
}

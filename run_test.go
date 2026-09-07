package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testRules = []Rule{
	{ID: "aaaa0001", Title: "Use httpx", Text: "# Use httpx\n\nEvery HTTP call goes through httpx."},
	{ID: "aaaa0002", Title: "No prints", Text: "# No prints\n\nUse the logger, not print."},
}

func TestPrompt(t *testing.T) {
	got := prompt(testRules, Scope{Base: "origin/main"}, "+import requests")
	for _, want := range []string{
		`<rule id="aaaa0001">`, `<rule id="aaaa0002">`,
		"Every HTTP call goes through httpx.",
		"the changes since origin/main",
		"<changes>\n+import requests\n</changes>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
}

func TestParseFindings(t *testing.T) {
	// The second rule is missing from the answer: it must fail, not vanish.
	text := `{"results":[{"id":"aaaa0001","pass":false,"reason":"main.py imports requests","files":["main.py"]}]}`
	got, err := parseFindings(text, testRules)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2", len(got))
	}
	if got[0].Pass || got[0].Files[0] != "main.py" {
		t.Errorf("first finding: %+v", got[0])
	}
	if got[1].ID != "aaaa0002" || got[1].Pass {
		t.Errorf("missing rule did not fail: %+v", got[1])
	}
	if !(Report{Findings: got}).Failed() {
		t.Error("report with a failure reports no failure")
	}
	if got, err := parseFindings("Here it is:\n```json\n{\"results\":[]}\n```\nDone {see above}.", testRules); err != nil || len(got) != len(testRules) {
		t.Errorf("fenced JSON: %v %v", got, err)
	}
	if _, err := parseFindings("not json", testRules); err == nil {
		t.Error("expected an error for a non-JSON answer")
	}
}

func TestParseScope(t *testing.T) {
	tests := []struct {
		args []string
		want Scope
		bad  bool
	}{
		{nil, Scope{}, false},
		{[]string{"--all"}, Scope{All: true}, false},
		{[]string{"--base", "main"}, Scope{Base: "main"}, false},
		{[]string{"--base=main"}, Scope{Base: "main"}, false},
		{[]string{"--commit", "abc"}, Scope{Commit: "abc"}, false},
		{[]string{"--uncommitted"}, Scope{Uncommitted: true}, false},
		{[]string{"--uncommitted", "--all"}, Scope{}, true},
		{[]string{"--base"}, Scope{}, true},
		{[]string{"--all", "--commit", "abc"}, Scope{}, true},
		{[]string{"--bogus"}, Scope{}, true},
	}
	for _, tt := range tests {
		got, err := parseScope(tt.args)
		if (err != nil) != tt.bad {
			t.Errorf("parseScope(%v) err = %v, want bad=%v", tt.args, err, tt.bad)
		}
		if !tt.bad && got != tt.want {
			t.Errorf("parseScope(%v) = %+v, want %+v", tt.args, got, tt.want)
		}
	}
}

func TestOpenAIProvider(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("bad request: %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"results\":[]}"}}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`))
	}))
	defer srv.Close()
	t.Setenv("OPENAI_API_KEY", "k")
	p := openaiProvider{cfg: Config{Model: "m", BaseURL: srv.URL + "/v1/", APIKeyEnv: "OPENAI_API_KEY", MaxOutputTokens: 9}}
	text, in, out, err := p.complete(context.Background(), "hello")
	if err != nil || text != `{"results":[]}` || in != 7 || out != 3 {
		t.Fatalf("got %q %d %d %v", text, in, out, err)
	}
	if got["model"] != "m" || got["max_tokens"] != 9.0 || got["response_format"] == nil {
		t.Errorf("request body: %v", got)
	}
}

func TestAnthropicProvider(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("bad headers: %v", r.Header)
		}
		switch r.URL.Path {
		case "/v1/messages/count_tokens":
			w.Write([]byte(`{"input_tokens":5}`))
		case "/v1/messages":
			json.NewDecoder(r.Body).Decode(&got)
			w.Write([]byte(`{"content":[{"type":"text","text":"{\"results\":[]}"}],"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":3}}`))
		default:
			t.Errorf("bad path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	t.Setenv("ANTHROPIC_API_KEY", "k")
	p := anthropicProvider{cfg: Config{Model: "m", BaseURL: srv.URL + "/", APIKeyEnv: "ANTHROPIC_API_KEY", MaxOutputTokens: 9}}
	if n, err := p.countTokens(context.Background(), "hello"); err != nil || n != 5 {
		t.Fatalf("count: got %d %v", n, err)
	}
	text, in, out, err := p.complete(context.Background(), "hello")
	if err != nil || text != `{"results":[]}` || in != 7 || out != 3 {
		t.Fatalf("got %q %d %d %v", text, in, out, err)
	}
	if got["model"] != "m" || got["max_tokens"] != 9.0 || got["output_config"] == nil {
		t.Errorf("request body: %v", got)
	}
}

func TestClaudeProvider(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\ncat > " + filepath.Join(dir, "in") + "\necho \"${ANTHROPIC_API_KEY-unset}\" > " + filepath.Join(dir, "key") + "\necho '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"ignored\",\"structured_output\":{\"results\":[]},\"usage\":{\"input_tokens\":2,\"cache_read_input_tokens\":5,\"output_tokens\":3}}'\n"
	os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ANTHROPIC_API_KEY", "stale")
	p, err := newProvider(Config{Provider: "claude", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	text, in, out, err := p.complete(context.Background(), "hello")
	if err != nil || text != `{"results":[]}` || in != 7 || out != 3 {
		t.Fatalf("got %q %d %d %v", text, in, out, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "in")); string(got) != "hello" {
		t.Errorf("stdin: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "key")); string(got) != "unset\n" {
		t.Errorf("the command got ANTHROPIC_API_KEY: %q", got)
	}
}

func TestCopilotProvider(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\ncat > " + filepath.Join(dir, "in") + "\necho \"$@\" > " + filepath.Join(dir, "args") + "\necho '{\"results\":[]}'\n"
	os.WriteFile(filepath.Join(dir, "copilot"), []byte(script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	p, err := newProvider(Config{Provider: "copilot", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	text, in, out, err := p.complete(context.Background(), "hello")
	if err != nil || text != "{\"results\":[]}\n" || in != 1 || out != 3 {
		t.Fatalf("got %q %d %d %v", text, in, out, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "in")); string(got) != "hello" {
		t.Errorf("stdin: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "args")); !strings.Contains(string(got), "--model m --available-tools=none") {
		t.Errorf("args: %q", got)
	}
}

type fakeProvider struct{}

func (fakeProvider) countTokens(context.Context, string) (int64, error) { return 1, nil }
func (fakeProvider) complete(context.Context, string) (string, int64, int64, error) {
	return `{"results":[{"id":"aaaa0001","pass":true,"reason":"ok","files":[]}]}`, 7, 3, nil
}

func TestAskPerRuleTokens(t *testing.T) {
	cfg := Config{Model: "m", MaxTokens: 100}
	report, err := ask(context.Background(), fakeProvider{}, cfg, testRules[:1], Scope{}, "+x")
	if err != nil {
		t.Fatal(err)
	}
	if report.Model != "m" || report.InputTokens != 7 || report.OutputTokens != 3 {
		t.Errorf("report: %+v", report)
	}
	if got := tokensNote(report.Findings[0]); got != "  (7 in, 3 out)" {
		t.Errorf("tokensNote = %q", got)
	}
	// Two rules share one request: no per-rule tokens.
	report, _ = ask(context.Background(), fakeProvider{}, cfg, testRules, Scope{}, "+x")
	if tokensNote(report.Findings[0]) != "" {
		t.Errorf("shared request got per-rule tokens: %+v", report.Findings[0])
	}
}

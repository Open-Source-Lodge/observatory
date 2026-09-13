package observatory

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
	one := `{"id":"aaaa0001","pass":true,"reason":"ok","files":[]}`
	for name, answer := range map[string]string{
		"fenced":     "Here it is:\n```json\n{\"results\":[" + one + "]}\n```\nDone {see above}.",
		"bare array": "[" + one + "]",
		"one object": one,
		"other key":  `{"verdicts":{"first":` + one + `}}`,
		// A model that makes up a tool call writes JSON that is not a
		// verdict. The verdict after it must still count.
		"made-up tool call": `{"cmd":"sed -n '1,120p' README.md"} to=functions.bash ` +
			`{"cmd":"nl -ba README.md"}` + "\n" + `{"results":[` + one + `]}`,
	} {
		got, err := parseFindings(answer, testRules)
		if err != nil || len(got) != 2 || !got[0].Pass || got[1].Pass {
			t.Errorf("%s: %v %v", name, got, err)
		}
	}
	if _, err := parseFindings(`{"results":[]}`, testRules); err == nil || !strings.Contains(err.Error(), `{"results":[]}`) {
		t.Errorf("an answer with no verdict must be an error that shows the answer, got %v", err)
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
		got, err := ParseScope(tt.args)
		if (err != nil) != tt.bad {
			t.Errorf("ParseScope(%v) err = %v, want bad=%v", tt.args, err, tt.bad)
		}
		if !tt.bad && got != tt.want {
			t.Errorf("ParseScope(%v) = %+v, want %+v", tt.args, got, tt.want)
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
	text, in, out, err := p.complete(context.Background(), "hello", findingsSchema())
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
		if r.URL.Path != "/v1/messages" {
			t.Errorf("bad path: %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"content":[{"type":"text","text":"{\"results\":[]}"}],"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":3}}`))
	}))
	defer srv.Close()
	t.Setenv("ANTHROPIC_API_KEY", "k")
	p := anthropicProvider{cfg: Config{Model: "m", BaseURL: srv.URL + "/", APIKeyEnv: "ANTHROPIC_API_KEY", MaxOutputTokens: 9}}
	text, in, out, err := p.complete(context.Background(), "hello", findingsSchema())
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
	text, in, out, err := p.complete(context.Background(), "hello", findingsSchema())
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
	text, in, out, err := p.complete(context.Background(), "hello", findingsSchema())
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

func (fakeProvider) complete(context.Context, string, map[string]any) (string, int64, int64, error) {
	return `{"results":[{"id":"aaaa0001","pass":true,"reason":"ok","files":[]}]}`, 7, 3, nil
}

func TestAskPerRuleTokens(t *testing.T) {
	cfg := Config{Model: "m", MaxTokens: 100000}
	report, err := ask(context.Background(), fakeProvider{}, cfg, testRules[:1], Scope{}, "+x")
	if err != nil {
		t.Fatal(err)
	}
	if report.Model != "m" || report.InputTokens != 7 || report.OutputTokens != 3 {
		t.Errorf("report: %+v", report)
	}
	if got := report.Findings[0].TokensNote(); got != "  (7 in, 3 out)" {
		t.Errorf("TokensNote = %q", got)
	}
	// The estimate guards max_tokens before the request.
	small := Config{Model: "m", MaxTokens: 1}
	if _, err := ask(context.Background(), fakeProvider{}, small, testRules[:1], Scope{}, "+x"); err == nil || !strings.Contains(err.Error(), "the limit is 1") {
		t.Errorf("max_tokens guard gave %v", err)
	}
	// Two rules share one request: no per-rule tokens.
	report, _ = ask(context.Background(), fakeProvider{}, cfg, testRules, Scope{}, "+x")
	if report.Findings[0].TokensNote() != "" {
		t.Errorf("shared request got per-rule tokens: %+v", report.Findings[0])
	}
}

func TestDraft(t *testing.T) {
	got := draftPrompt(Draft{Title: "No prints"})
	if !strings.Contains(got, "<title>No prints</title>") || !strings.Contains(got, "<rule></rule>") {
		t.Errorf("prompt lacks the fields: %s", got)
	}
	d, err := parseDraft("Here:\n```json\n{\"title\":\"No prints\",\"rule\":\"Use the logger.\",\"why\":\"Logs have levels.\"}\n```")
	if err != nil || d.Rule != "Use the logger." || d.Why != "Logs have levels." {
		t.Errorf("parseDraft: %+v %v", d, err)
	}
	if _, err := parseDraft("not json"); err == nil {
		t.Error("expected an error for a non-JSON answer")
	}
}

func TestCheckIgnore(t *testing.T) {
	Status = func(string) func() { return func() {} }
	rules := []Rule{
		{ID: "aaaa0002", Title: "No prints", Text: "x", Ignore: []string{"*.py"}},
		{ID: "aaaa0001", Title: "Use httpx", Text: "x"},
	}
	if got := batches(rules, false); len(got) != 2 || len(got[0]) != 1 || got[0][0].ID != "aaaa0002" {
		t.Errorf("batches: %v", got)
	}
	if got := batches(testRules, false); len(got) != 1 || len(got[0]) != 2 {
		t.Errorf("batches without ignore: %v", got)
	}
	if got := batches(testRules, true); len(got) != 2 {
		t.Errorf("batches per rule: %v", got)
	}
	newProvider = func(Config) (provider, error) { return fakeProvider{}, nil }
	show := "show --format=commit %H%n%s%n%n%b --patch HEAD"
	stubGit(t, map[string]string{
		// The pathspecs of the first rule leave no file, so git answers nothing.
		show + " -- :(exclude,top,glob)**/*.py :(exclude,top,glob)**/*.py/**": "",
		show: "diff --git a/main.py b/main.py\n+print(1)\n",
	})
	report, err := Check(context.Background(), Config{Model: "m", MaxTokens: 100000}, rules, Scope{Commit: "HEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 2 || report.Findings[0].ID != "aaaa0002" || !report.Findings[0].Pass || report.Findings[0].Reason != "the rule ignores every file in the changes" {
		t.Errorf("ignored rule: %+v", report.Findings)
	}
	if report.Findings[1].ID != "aaaa0001" || !report.Findings[1].Pass || report.InputTokens != 7 {
		t.Errorf("asked rule: %+v %d", report.Findings, report.InputTokens)
	}
}

// TestCheckEmptyScope: a scope with no change at all is an error, not a
// silent pass.
func TestCheckEmptyScope(t *testing.T) {
	Status = func(string) func() { return func() {} }
	newProvider = func(Config) (provider, error) { return fakeProvider{}, nil }
	stubGit(t, map[string]string{"show --format=commit %H%n%s%n%n%b --patch HEAD": ""})
	_, err := Check(context.Background(), Config{Model: "m", MaxTokens: 100000}, testRules, Scope{Commit: "HEAD"})
	if err == nil || !strings.Contains(err.Error(), "nothing to check") {
		t.Errorf("empty scope gave %v", err)
	}
}

// TestCheckEmptyScopeGitError: the second call to git decides between an
// empty scope and a rule that ignores everything. Its error must reach the
// user, and not become "nothing to check".
func TestCheckEmptyScopeGitError(t *testing.T) {
	Status = func(string) func() { return func() {} }
	newProvider = func(Config) (provider, error) { return fakeProvider{}, nil }
	rules := []Rule{{ID: "aaaa0001", Title: "Use httpx", Text: "x", Ignore: []string{"*.py"}}}
	show := "show --format=commit %H%n%s%n%n%b --patch HEAD"
	// The call with the pathspecs answers. The call without them does not.
	stubGit(t, map[string]string{
		show + " -- :(exclude,top,glob)**/*.py :(exclude,top,glob)**/*.py/**": "",
	})
	_, err := Check(context.Background(), Config{Model: "m", MaxTokens: 100000}, rules, Scope{Commit: "HEAD"})
	if err == nil || strings.Contains(err.Error(), "nothing to check") {
		t.Errorf("the git error did not reach the caller: %v", err)
	}
}

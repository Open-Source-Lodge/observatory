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

func TestClaudeProvider(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\ncat > " + filepath.Join(dir, "in") + "\necho '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"ignored\",\"structured_output\":{\"results\":[]},\"usage\":{\"input_tokens\":2,\"cache_read_input_tokens\":5,\"output_tokens\":3}}'\n"
	os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
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
}

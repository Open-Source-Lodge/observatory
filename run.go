package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// status shows progress on stderr while a request runs, so a slow model does
// not look like a hang. A terminal gets a spinner with the elapsed time; a
// log gets one line. The report goes to stdout. stop ends the spinner.
var status = func(text string) (stop func()) {
	if fi, err := os.Stderr.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		fmt.Fprintln(os.Stderr, text+" ...")
		return func() {}
	}
	done, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
		start := time.Now()
		for i := 0; ; i++ {
			fmt.Fprintf(os.Stderr, "\r\033[K%c %s %ds", frames[i%len(frames)], text, int(time.Since(start).Seconds()))
			select {
			case <-done:
				fmt.Fprint(os.Stderr, "\r\033[K")
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()
	return func() { close(done); <-finished }
}

// Finding is the verdict of the model on one rule.
type Finding struct {
	ID     string   `json:"id"`
	Pass   bool     `json:"pass"`
	Reason string   `json:"reason"`
	Files  []string `json:"files"`
	// InputTokens and OutputTokens are what this rule cost. They have a
	// value only when the rule had its own request (per_rule).
	InputTokens  int64 `json:"-"`
	OutputTokens int64 `json:"-"`
}

// Report is the outcome of one run.
type Report struct {
	Scope    string
	Model    string
	Findings []Finding
	// InputTokens and OutputTokens are what the run cost.
	InputTokens  int64
	OutputTokens int64
}

// Failed reports whether any rule failed.
func (r Report) Failed() bool {
	for _, f := range r.Findings {
		if !f.Pass {
			return true
		}
	}
	return false
}

// check asks the model for a finding per rule: one request for all the
// rules, or one request per rule when the config says per_rule.
func check(ctx context.Context, cfg Config, rules []Rule, scope Scope, diff string) (Report, error) {
	if len(rules) == 0 {
		return Report{}, errors.New("no rules — add one with 'observatory add <title>'")
	}
	if strings.TrimSpace(diff) == "" {
		return Report{}, errors.New("nothing to check: " + scope.String() + " is empty")
	}
	p, err := newProvider(cfg)
	if err != nil {
		return Report{}, err
	}
	head := fmt.Sprintf("check of %s with %s %s", scope, cfg.Provider, cfg.Model)
	if !cfg.PerRule {
		stop := status(head + ", " + rulesLabel(rules))
		defer stop()
		return ask(ctx, p, cfg, rules, scope, diff)
	}
	report := Report{Scope: scope.String(), Model: cfg.Model}
	for i, r := range rules {
		stop := status(fmt.Sprintf("%s, %d/%d %s", head, i+1, len(rules), rulesLabel([]Rule{r})))
		one, err := ask(ctx, p, cfg, []Rule{r}, scope, diff)
		stop()
		if err != nil {
			return report, fmt.Errorf("rule %s: %w", r.ID, err)
		}
		report.Findings = append(report.Findings, one.Findings...)
		report.InputTokens += one.InputTokens
		report.OutputTokens += one.OutputTokens
	}
	return report, nil
}

// rulesLabel names the rules of a request: one rule with its title, or the
// IDs of many.
func rulesLabel(rules []Rule) string {
	if len(rules) == 1 {
		return "rule " + rules[0].ID + " " + rules[0].Title
	}
	ids := make([]string, len(rules))
	for i, r := range rules {
		ids[i] = r.ID
	}
	return fmt.Sprintf("%d rules %s", len(rules), strings.Join(ids, ", "))
}

// ask sends rules and the changes to the provider in one request and reads
// back one finding per rule.
func ask(ctx context.Context, p provider, cfg Config, rules []Rule, scope Scope, diff string) (Report, error) {
	report := Report{Scope: scope.String(), Model: cfg.Model}
	text := prompt(rules, scope, diff)
	count, err := p.countTokens(ctx, text)
	if err != nil {
		return report, fmt.Errorf("count tokens: %w", err)
	}
	if count > int64(cfg.MaxTokens) {
		return report, fmt.Errorf("the prompt is %d tokens, the limit is %d — narrow the scope, or raise max_tokens in %s/config",
			count, cfg.MaxTokens, rulesDirName)
	}
	answer, in, out, err := p.complete(ctx, text, findingsSchema())
	if err != nil {
		return report, err
	}
	report.InputTokens, report.OutputTokens = in, out
	report.Findings, err = parseFindings(answer, rules)
	if len(rules) == 1 && len(report.Findings) == 1 {
		report.Findings[0].InputTokens, report.Findings[0].OutputTokens = in, out
	}
	return report, err
}

// prompt is the whole request: the rules, the scope, the changes, and what
// to answer. Everything is in one user message so that count_tokens and the
// request measure the same text.
func prompt(rules []Rule, scope Scope, diff string) string {
	var b strings.Builder
	b.WriteString("You review changes to a code repository against the rules of that repository.\n")
	b.WriteString("For each rule, decide whether the changes below break it. Judge only what the changes show; ")
	b.WriteString("a rule that the changes do not touch passes. Quote the file and the line that breaks a rule in the reason.\n\n")
	b.WriteString("Answer with JSON: {\"results\": [{\"id\", \"pass\", \"reason\", \"files\"}]}, one entry per rule, in the order given. ")
	b.WriteString("Keep each reason to one or two sentences.\n\n")
	b.WriteString("# Rules\n\n")
	for _, r := range rules {
		fmt.Fprintf(&b, "<rule id=\"%s\">\n%s\n</rule>\n\n", r.ID, r.Text)
	}
	fmt.Fprintf(&b, "# Changes: %s\n\n<changes>\n%s\n</changes>\n", scope.String(), diff)
	return b.String()
}

func findingsSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"results": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":     map[string]any{"type": "string"},
						"pass":   map[string]any{"type": "boolean"},
						"reason": map[string]any{"type": "string"},
						"files":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"required":             []string{"id", "pass", "reason", "files"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"results"},
		"additionalProperties": false,
	}
}

// parseFindings reads the answer and returns one finding per rule, in rule
// order. A rule the model did not answer for fails, so that a silent miss
// cannot pass a check. An answer with no verdict at all is an error that
// shows the answer.
func parseFindings(text string, rules []Rule) ([]Finding, error) {
	// A model without a JSON schema can wrap the answer in a code fence, add
	// text around it, or leave out the {"results": ...} wrapper. Decode the
	// first JSON value and collect every object in it that has an id and a
	// pass.
	if i := strings.IndexAny(text, "{["); i > 0 {
		text = text[i:]
	}
	var value any
	if err := json.NewDecoder(strings.NewReader(text)).Decode(&value); err != nil {
		return nil, fmt.Errorf("the model did not answer with JSON: %w: %s", err, snippet(text))
	}
	byID := map[string]Finding{}
	collectFindings(value, byID)
	if len(byID) == 0 {
		return nil, errors.New("the model gave no verdict: " + snippet(text))
	}
	findings := make([]Finding, len(rules))
	for i, r := range rules {
		f, ok := byID[r.ID]
		if !ok {
			f = Finding{ID: r.ID, Reason: "the model gave no verdict for this rule"}
		}
		findings[i] = f
	}
	return findings, nil
}

// collectFindings walks a decoded JSON value and puts every object with a
// string id and a boolean pass into byID.
func collectFindings(value any, byID map[string]Finding) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			collectFindings(item, byID)
		}
	case map[string]any:
		id, hasID := v["id"].(string)
		_, hasPass := v["pass"].(bool)
		if hasID && hasPass {
			var f Finding
			data, _ := json.Marshal(v)
			if json.Unmarshal(data, &f) == nil {
				byID[id] = f
			}
			return
		}
		for _, item := range v {
			collectFindings(item, byID)
		}
	}
}

// snippet is the start of an answer, for an error message.
func snippet(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 300 {
		text = text[:300] + "..."
	}
	return text
}

// Draft is what the model writes for a new rule.
type Draft struct {
	Title string `json:"title"`
	Rule  string `json:"rule"`
	Why   string `json:"why"`
}

// draft asks the model to write the empty fields of a new rule. The fields
// that have a value stay as they are.
func draft(ctx context.Context, cfg Config, d Draft) (Draft, int64, int64, error) {
	p, err := newProvider(cfg)
	if err != nil {
		return d, 0, 0, err
	}
	answer, in, out, err := p.complete(ctx, draftPrompt(d), draftSchema())
	if err != nil {
		return d, in, out, err
	}
	got, err := parseDraft(answer)
	if err != nil {
		return d, in, out, err
	}
	// ponytail: the prompt asks the model to keep the given fields. This
	// guard makes sure of it.
	if strings.TrimSpace(d.Title) == "" {
		d.Title = strings.TrimSpace(got.Title)
	}
	if strings.TrimSpace(d.Rule) == "" {
		d.Rule = strings.TrimSpace(got.Rule)
	}
	if strings.TrimSpace(d.Why) == "" {
		d.Why = strings.TrimSpace(got.Why)
	}
	return d, in, out, nil
}

// draftPrompt is the request for a draft: what a rule is, the fields the
// user gave, and what to answer.
func draftPrompt(d Draft) string {
	var b strings.Builder
	b.WriteString("You write a rule for observatory, a tool that sends rules in plain language and a code change to a model, ")
	b.WriteString("and the model says whether the change breaks each rule.\n\n")
	b.WriteString("A rule has three fields. The title is one short line. The rule is one or two exact sentences that a model ")
	b.WriteString("can check against a diff; it goes into the prompt of each check, so keep it short. The why is one or two ")
	b.WriteString("sentences for people: the reason for the rule and the business context.\n\n")
	b.WriteString("Example: title \"Use httpx for HTTP calls\", rule \"Every HTTP call goes through httpx; do not import ")
	b.WriteString("requests or urllib.\", why \"One HTTP client keeps the retry and timeout settings in one place.\"\n\n")
	b.WriteString("The user gave these fields. Write the empty ones. Return the given ones unchanged.\n\n")
	fmt.Fprintf(&b, "<title>%s</title>\n<rule>%s</rule>\n<why>%s</why>\n\n", d.Title, d.Rule, d.Why)
	b.WriteString("Answer with JSON: {\"title\", \"rule\", \"why\"}.\n")
	return b.String()
}

func draftSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
			"rule":  map[string]any{"type": "string"},
			"why":   map[string]any{"type": "string"},
		},
		"required":             []string{"title", "rule", "why"},
		"additionalProperties": false,
	}
}

// parseDraft reads the answer. A model without a JSON schema can wrap the
// answer in a code fence or add text around it.
func parseDraft(text string) (Draft, error) {
	if i := strings.Index(text, "{"); i > 0 {
		text = text[i:]
	}
	var d Draft
	if err := json.NewDecoder(strings.NewReader(text)).Decode(&d); err != nil {
		return d, fmt.Errorf("the model did not answer with JSON: %w: %s", err, snippet(text))
	}
	return d, nil
}

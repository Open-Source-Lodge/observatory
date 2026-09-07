package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Finding is the verdict of the model on one rule.
type Finding struct {
	ID     string   `json:"id"`
	Pass   bool     `json:"pass"`
	Reason string   `json:"reason"`
	Files  []string `json:"files"`
}

// Report is the outcome of one run.
type Report struct {
	Scope    string
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
	if !cfg.PerRule {
		return ask(ctx, p, cfg, rules, scope, diff)
	}
	report := Report{Scope: scope.String()}
	for _, r := range rules {
		one, err := ask(ctx, p, cfg, []Rule{r}, scope, diff)
		if err != nil {
			return report, fmt.Errorf("rule %s: %w", r.ID, err)
		}
		report.Findings = append(report.Findings, one.Findings...)
		report.InputTokens += one.InputTokens
		report.OutputTokens += one.OutputTokens
	}
	return report, nil
}

// ask sends rules and the changes to the provider in one request and reads
// back one finding per rule.
func ask(ctx context.Context, p provider, cfg Config, rules []Rule, scope Scope, diff string) (Report, error) {
	report := Report{Scope: scope.String()}
	text := prompt(rules, scope, diff)
	count, err := p.countTokens(ctx, text)
	if err != nil {
		return report, fmt.Errorf("count tokens: %w", err)
	}
	if count > int64(cfg.MaxTokens) {
		return report, fmt.Errorf("the prompt is %d tokens, the limit is %d — narrow the scope, or raise max_tokens in %s/config",
			count, cfg.MaxTokens, rulesDirName)
	}
	answer, in, out, err := p.complete(ctx, text)
	if err != nil {
		return report, err
	}
	report.InputTokens, report.OutputTokens = in, out
	report.Findings, err = parseFindings(answer, rules)
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
// cannot pass a check.
func parseFindings(text string, rules []Rule) ([]Finding, error) {
	var out struct {
		Results []Finding `json:"results"`
	}
	// A model without a JSON schema can wrap the answer in a code fence, or
	// add text around it. Decode the first JSON object and ignore the rest.
	if i := strings.Index(text, "{"); i > 0 {
		text = text[i:]
	}
	if err := json.NewDecoder(strings.NewReader(text)).Decode(&out); err != nil {
		return nil, fmt.Errorf("the model did not answer with JSON: %w", err)
	}
	byID := make(map[string]Finding, len(out.Results))
	for _, f := range out.Results {
		byID[f.ID] = f
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

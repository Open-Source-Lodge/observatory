package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

func cmdInit(args []string) error {
	if len(args) > 0 {
		return errors.New("init takes no arguments")
	}
	dir, err := rulesDir()
	if err != nil {
		return err
	}
	created, err := initRepo(dir)
	if err != nil {
		return err
	}
	if created {
		fmt.Println("made " + filepath.Join(dir, "config"))
	} else {
		fmt.Println(filepath.Join(dir, "config") + " already exists")
	}
	return nil
}

func cmdList(args []string) error {
	if len(args) > 0 {
		return errors.New("list takes no arguments")
	}
	dir, err := rulesDir()
	if err != nil {
		return err
	}
	rules, err := loadRules(dir)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tRULE")
	for _, r := range rules {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.ID, r.Title, r.Summary())
	}
	return w.Flush()
}

func cmdAdd(args []string) error {
	title := strings.TrimSpace(strings.Join(args, " "))
	if title == "" {
		return errors.New("usage: observatory add <title>")
	}
	dir, err := rulesDir()
	if err != nil {
		return err
	}
	r, err := createRule(dir, title, "", "")
	if err != nil {
		return err
	}
	fmt.Println(r.Dir)
	fmt.Println("write the rule in rule.md, and the reasons for it in explain.md")
	return nil
}

func cmdRun(args []string) error {
	scope, err := parseScope(args)
	if err != nil {
		return err
	}
	report, err := runCheck(context.Background(), scope)
	if err != nil {
		return err
	}
	printReport(report)
	if report.Failed() {
		os.Exit(1)
	}
	return nil
}

// runCheck loads the rules and the changes, and asks the model.
func runCheck(ctx context.Context, scope Scope) (Report, error) {
	dir, err := rulesDir()
	if err != nil {
		return Report{}, err
	}
	rules, err := loadRules(dir)
	if err != nil {
		return Report{}, err
	}
	diff, err := changes(&scope)
	if err != nil {
		return Report{}, err
	}
	return check(ctx, loadConfig(dir), rules, scope, diff)
}

// printReport writes one line per rule, and the cost. In GitHub Actions it
// also writes an annotation per failure, so the failure shows on the file.
func printReport(r Report) {
	fmt.Println("model: " + r.Model)
	fmt.Println("checked " + r.Scope)
	for _, f := range r.Findings {
		mark := "PASS"
		if !f.Pass {
			mark = "FAIL"
		}
		fmt.Printf("%s  %s  %s%s\n", mark, f.ID, f.Reason, tokensNote(f))
		if !f.Pass && os.Getenv("GITHUB_ACTIONS") != "" {
			file := ""
			if len(f.Files) > 0 {
				file = " file=" + f.Files[0]
			}
			fmt.Printf("::error%s::rule %s: %s\n", file, f.ID, f.Reason)
		}
	}
	fmt.Printf("tokens: %d in, %d out\n", r.InputTokens, r.OutputTokens)
}

// tokensNote is the cost of one rule, or empty when the rule shared its
// request with the other rules.
func tokensNote(f Finding) string {
	if f.InputTokens == 0 && f.OutputTokens == 0 {
		return ""
	}
	return fmt.Sprintf("  (%d in, %d out)", f.InputTokens, f.OutputTokens)
}

func cmdDoctor(args []string) error {
	if len(args) > 0 {
		return errors.New("doctor takes no arguments")
	}
	ok := true
	report := func(name string, err error) {
		if err != nil {
			ok = false
			fmt.Printf("✗ %s: %s\n", name, err)
			return
		}
		fmt.Printf("✓ %s\n", name)
	}
	_, err := exec.LookPath("git")
	report("git is installed", err)
	dir, err := rulesDir()
	report("inside a git repository", err)
	if err == nil {
		_, err := os.Stat(dir)
		report(rulesDirName+" exists", err)
		rules, err := loadRules(dir)
		if err == nil && len(rules) == 0 {
			err = errors.New("no rules yet — add one with 'observatory add <title>'")
		}
		report("rules load", err)
	}
	cfg := loadConfig(dir)
	_, err = newProvider(cfg)
	report(fmt.Sprintf("provider %s, model %s", cfg.Provider, cfg.Model), err)
	switch {
	case cfg.Provider == "claude", cfg.Provider == "copilot":
		// The command holds its own login.
	case os.Getenv(cfg.APIKeyEnv) == "" && (cfg.Provider != "anthropic" || os.Getenv("ANTHROPIC_AUTH_TOKEN") == ""):
		report("API credentials", errors.New(cfg.APIKeyEnv+" is not set"))
	default:
		report("API credentials", nil)
	}
	if !ok {
		return errors.New("some checks failed")
	}
	return nil
}

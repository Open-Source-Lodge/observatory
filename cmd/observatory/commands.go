package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/Open-Source-Lodge/observatory/internal/observatory"
)

func cmdInit(args []string) error {
	if len(args) > 0 {
		return errors.New("init takes no arguments")
	}
	dir, err := observatory.RulesDir()
	if err != nil {
		return err
	}
	created, err := observatory.InitRepo(dir)
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
	dir, err := observatory.RulesDir()
	if err != nil {
		return err
	}
	rules, err := observatory.LoadRules(dir)
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
	dir, err := observatory.RulesDir()
	if err != nil {
		return err
	}
	r, err := observatory.CreateRule(dir, title, "", "")
	if err != nil {
		return err
	}
	fmt.Println(r.Dir)
	fmt.Println("write the rule in rule.md, and the reasons for it in explain.md")
	return nil
}

func cmdRun(args []string) error {
	scope, err := observatory.ParseScope(args)
	if err != nil {
		return err
	}
	report, err := runCheck(context.Background(), scope, nil)
	if err != nil {
		return err
	}
	printReport(report)
	if report.Failed() {
		os.Exit(1)
	}
	return nil
}

// runCheck loads the changes and asks the model about rules. A nil rules
// loads every rule.
func runCheck(ctx context.Context, scope observatory.Scope, rules []observatory.Rule) (observatory.Report, error) {
	dir, err := observatory.RulesDir()
	if err != nil {
		return observatory.Report{}, err
	}
	if rules == nil {
		if rules, err = observatory.LoadRules(dir); err != nil {
			return observatory.Report{}, err
		}
	}
	return observatory.Check(ctx, observatory.LoadConfig(dir), rules, scope)
}

// printReport writes one line per rule, and the cost. In GitHub Actions it
// also writes an annotation per failure, so the failure shows on the file.
func printReport(r observatory.Report) {
	fmt.Println("model: " + r.Model)
	fmt.Println("checked " + r.Scope)
	for _, f := range r.Findings {
		mark := "PASS"
		if !f.Pass {
			mark = "FAIL"
		}
		fmt.Printf("%s  %s  %s%s\n", mark, f.ID, f.Reason, f.TokensNote())
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

package observatory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Rule is one directory under `.observatory`. The directory name is the ID.
type Rule struct {
	ID    string
	Title string
	// Text is the full content of rule.md, which goes into the prompt.
	Text string
	// Ignore is the content of the file `ignore` in the directory of the
	// rule: the files, the directories and the globs that the rule does
	// not see. One pattern per line, like a .gitignore file.
	Ignore []string
	Dir    string
}

// Summary is the first line of the rule after the title, for a list.
func (r Rule) Summary() string {
	for _, line := range strings.Split(r.Text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

// LoadRules reads every `<dir>/<id>/rule.md` and `<dir>/<id>/ignore`,
// sorted by ID. A directory without a rule.md is not a rule, and LoadRules
// ignores it.
func LoadRules(dir string) ([]Rule, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("no %s directory — run 'observatory init'", rulesDirName)
		}
		return nil, err
	}
	var rules []Rule
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ruleDir := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(filepath.Join(ruleDir, "rule.md"))
		if err != nil {
			continue
		}
		rules = append(rules, Rule{
			ID:     e.Name(),
			Title:  titleOf(string(data), e.Name()),
			Text:   strings.TrimSpace(string(data)),
			Ignore: patterns(filepath.Join(ruleDir, "ignore")),
			Dir:    ruleDir,
		})
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules, nil
}

// patterns reads an ignore file: one pattern per line, with `#` comments.
// A missing file gives no patterns.
func patterns(path string) []string {
	data, _ := os.ReadFile(path)
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// titleOf is the first `# ` heading of a markdown document, or fallback.
func titleOf(md, fallback string) string {
	for _, line := range strings.Split(md, "\n") {
		if t, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(t)
		}
	}
	return fallback
}

// CreateRule makes `<dir>/<id>/` with rule.md and explain.md. The text and
// the explanation are optional; the files then hold only the title.
func CreateRule(dir, title, text, explain string) (Rule, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Rule{}, errors.New("a rule needs a title")
	}
	id, err := newID(dir)
	if err != nil {
		return Rule{}, err
	}
	ruleDir := filepath.Join(dir, id)
	if err := os.MkdirAll(ruleDir, 0o755); err != nil {
		return Rule{}, err
	}
	rule := "# " + title + "\n\n" + strings.TrimSpace(text) + "\n"
	if err := os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte(rule), 0o644); err != nil {
		return Rule{}, err
	}
	expl := "# " + title + "\n\n" + strings.TrimSpace(explain) + "\n"
	if err := os.WriteFile(filepath.Join(ruleDir, "explain.md"), []byte(expl), 0o644); err != nil {
		return Rule{}, err
	}
	return Rule{ID: id, Title: title, Text: strings.TrimSpace(rule), Dir: ruleDir}, nil
}

// idPrefix starts every rule ID: OBS-001, OBS-002, and so on.
const idPrefix = "OBS-"

// newID is one more than the highest ID in dir.
//
// newID makes the next sequential ID. Sequential IDs are easy to read, but
// two branches that each add a rule get the same ID. Rename one directory
// when that merge occurs.
func newID(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	highest := 0
	for _, e := range entries {
		n, err := strconv.Atoi(strings.TrimPrefix(e.Name(), idPrefix))
		if e.IsDir() && err == nil && n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("%s%03d", idPrefix, highest+1), nil
}

// DeleteRule removes the directory of the rule.
func DeleteRule(r Rule) error {
	if r.Dir == "" || r.ID == "" {
		return errors.New("the rule has no directory, so there is nothing to delete")
	}
	return os.RemoveAll(r.Dir)
}

// InitRepo makes `<dir>` with a config file. It leaves an existing config alone.
func InitRepo(dir string) (created bool, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	path := filepath.Join(dir, "config")
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(configTemplate), 0o644)
}

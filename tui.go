package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(9)
)

const (
	listHelp   = "↑↓ move · n new · e editor · d delete · r run · R run all · ctrl+r refresh · q quit"
	newHelp    = "tab next field · enter create · esc cancel"
	deleteHelp = "y delete · esc cancel"
	reportHelp = "esc back · q quit"
	busyHelp   = "working · ctrl+c quit"
)

type mode int

const (
	modeList mode = iota
	modeNew
	modeDelete
	modeReport
)

type rulesMsg struct {
	rules []Rule
	err   error
}

// doneMsg reports the outcome of an action that ran outside the update loop.
type doneMsg struct {
	text string
	err  error
}

type reportMsg struct {
	report Report
	err    error
}

type model struct {
	dir     string
	rules   []Rule
	cursor  int
	mode    mode
	inputs  []textinput.Model
	focus   int
	msg     string
	msgErr  bool
	busy    string
	spinner spinner.Model
	report  Report
}

func tui() error {
	dir, err := rulesDir()
	if err != nil {
		return err
	}
	status = func(string) func() { return func() {} } // the spinner of the screen shows the progress
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(cursorStyle))
	_, err = tea.NewProgram(model{dir: dir, spinner: sp}, tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd { return m.loadRules }

func (m model) loadRules() tea.Msg {
	rules, err := loadRules(m.dir)
	return rulesMsg{rules: rules, err: err}
}

func newInputs() []textinput.Model {
	mk := func(placeholder string, width int) textinput.Model {
		t := textinput.New()
		t.Placeholder = placeholder
		t.Width = width
		t.Prompt = ""
		return t
	}
	title := mk("Use httpx for HTTP calls", 60)
	title.Focus()
	return []textinput.Model{
		title,
		mk("Every HTTP call goes through httpx; do not import requests or urllib.", 80),
		mk("Why the rule exists (optional; you can write more in explain.md later)", 80),
	}
}

func createCmd(dir, title, text, explain string) tea.Cmd {
	return func() tea.Msg {
		r, err := createRule(dir, title, text, explain)
		if err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{text: "made rule " + r.ID}
	}
}

func deleteCmd(r Rule) tea.Cmd {
	return func() tea.Msg {
		if err := deleteRule(r); err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{text: "deleted rule " + r.ID}
	}
}

// runCmd checks the last commit against rules; nil means every rule.
func runCmd(rules []Rule) tea.Cmd {
	return func() tea.Msg {
		report, err := runCheck(context.Background(), Scope{}, rules)
		return reportMsg{report: report, err: err}
	}
}

// editorCmd opens the directory of the rule in the editor. A terminal editor
// gets the screen for as long as it runs; a windowed one returns at once.
func editorCmd(r Rule) tea.Cmd {
	argv := editorCommand()
	if len(argv) == 0 {
		return func() tea.Msg {
			return doneMsg{err: errors.New("no editor set — set OBSERVATORY_EDITOR, VISUAL or EDITOR")}
		}
	}
	args := append(append([]string{}, argv[1:]...), r.Dir)
	c := exec.Command(argv[0], args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{text: "opened " + r.ID + " in " + argv[0]}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case rulesMsg:
		m.rules = msg.rules
		if msg.err != nil {
			m.setMsg("", msg.err)
		}
		if m.cursor >= len(m.rules) {
			m.cursor = max(0, len(m.rules)-1)
		}

	case doneMsg:
		m.busy = ""
		m.setMsg(msg.text, msg.err)
		return m, m.loadRules

	case reportMsg:
		m.busy = ""
		if msg.err != nil {
			m.setMsg("", msg.err)
			return m, nil
		}
		m.report, m.mode = msg.report, modeReport

	case spinner.TickMsg:
		if m.busy == "" {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch m.mode {
		case modeNew:
			return m.updateNew(msg)
		case modeDelete:
			return m.updateDelete(msg)
		case modeReport:
			return m.updateReport(msg)
		default:
			return m.updateList(msg)
		}
	}
	if m.mode == modeNew {
		return m.updateInputs(msg)
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.busy != "" {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "up", "k", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j", "ctrl+n":
		if m.cursor < len(m.rules)-1 {
			m.cursor++
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = max(0, len(m.rules)-1)
	case "ctrl+r":
		m.setMsg("", nil)
		return m, m.loadRules
	case "r":
		if r, ok := m.selected(); ok {
			m.setMsg("", nil)
			return m.start("checking the changes against rule "+r.ID, runCmd([]Rule{r}))
		}
	case "n":
		m.mode, m.focus, m.inputs = modeNew, 0, newInputs()
		m.setMsg("", nil)
		return m, textinput.Blink
	case "e", "enter":
		if r, ok := m.selected(); ok {
			m.setMsg("", nil)
			return m, editorCmd(r)
		}
	case "d", "x":
		if _, ok := m.selected(); ok {
			m.mode = modeDelete
			m.setMsg("", nil)
		}
	case "R":
		m.setMsg("", nil)
		return m.start("checking the changes against every rule", runCmd(nil))
	}
	return m, nil
}

func (m model) updateNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = modeList
		return m, nil
	case "enter":
		title := strings.TrimSpace(m.inputs[0].Value())
		if title == "" {
			return m, nil
		}
		m.mode = modeList
		return m.start("making "+title, createCmd(m.dir, title, m.inputs[1].Value(), m.inputs[2].Value()))
	case "tab", "down", "shift+tab", "up":
		if msg.String() == "tab" || msg.String() == "down" {
			m.focus = (m.focus + 1) % len(m.inputs)
		} else {
			m.focus = (m.focus - 1 + len(m.inputs)) % len(m.inputs)
		}
		for i := range m.inputs {
			if i == m.focus {
				m.inputs[i].Focus()
			} else {
				m.inputs[i].Blur()
			}
		}
		return m, textinput.Blink
	}
	return m.updateInputs(msg)
}

func (m model) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m model) updateDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	r, ok := m.selected()
	if !ok {
		m.mode = modeList
		return m, nil
	}
	switch msg.String() {
	case "y", "enter":
		m.mode = modeList
		return m.start("deleting "+r.ID, deleteCmd(r))
	case "esc", "n", "q", "ctrl+c":
		m.mode = modeList
	}
	return m, nil
}

func (m model) updateReport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "enter":
		m.mode = modeList
	}
	return m, nil
}

// start marks the model busy with text and runs cmd.
func (m model) start(text string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.busy = text
	return m, tea.Batch(m.spinner.Tick, cmd)
}

func (m model) selected() (Rule, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rules) {
		return Rule{}, false
	}
	return m.rules[m.cursor], true
}

func (m *model) setMsg(text string, err error) {
	if err != nil {
		m.msg, m.msgErr = err.Error(), true
		return
	}
	m.msg, m.msgErr = text, false
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("  " + titleStyle.Render("observatory") + dimStyle.Render(" · "+m.dir) + "\n\n")
	switch m.mode {
	case modeNew:
		m.viewNew(&b)
	case modeReport:
		m.viewReport(&b)
	default:
		m.viewList(&b)
	}
	b.WriteString("\n")
	switch {
	case m.busy != "":
		b.WriteString("  " + m.spinner.View() + " " + m.busy + "\n")
	case m.msg != "" && m.msgErr:
		b.WriteString("  " + errStyle.Render(m.msg) + "\n")
	case m.msg != "":
		b.WriteString("  " + okStyle.Render(m.msg) + "\n")
	}
	b.WriteString("\n  " + dimStyle.Render(m.help()) + "\n")
	return b.String()
}

func (m model) help() string {
	switch {
	case m.busy != "":
		return busyHelp
	case m.mode == modeNew:
		return newHelp
	case m.mode == modeDelete:
		return deleteHelp
	case m.mode == modeReport:
		return reportHelp
	}
	return listHelp
}

func (m model) viewList(b *strings.Builder) {
	if len(m.rules) == 0 {
		b.WriteString("  " + dimStyle.Render("no rules yet — press n to add one") + "\n")
		return
	}
	for i, r := range m.rules {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("❯ ")
		}
		line := fmt.Sprintf("%s  %s", dimStyle.Render(r.ID), r.Title)
		if s := r.Summary(); s != "" {
			line += dimStyle.Render("  " + s)
		}
		b.WriteString(cursor + line + "\n")
	}
	if m.mode == modeDelete {
		if r, ok := m.selected(); ok {
			b.WriteString("\n  " + errStyle.Render("delete rule "+r.ID+" ("+r.Title+")?") + "\n")
		}
	}
}

func (m model) viewNew(b *strings.Builder) {
	b.WriteString("  " + titleStyle.Render("New rule") + "\n\n")
	for i, label := range []string{"title", "rule", "why"} {
		b.WriteString("  " + labelStyle.Render(label) + m.inputs[i].View() + "\n")
	}
}

func (m model) viewReport(b *strings.Builder) {
	b.WriteString("  " + titleStyle.Render("Checked "+m.report.Scope) + dimStyle.Render(" · "+m.report.Model) + "\n\n")
	for _, f := range m.report.Findings {
		mark := okStyle.Render("PASS")
		if !f.Pass {
			mark = errStyle.Render("FAIL")
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s%s\n", mark, dimStyle.Render(f.ID), f.Reason, dimStyle.Render(tokensNote(f))))
	}
	b.WriteString(fmt.Sprintf("\n  %s\n", dimStyle.Render(fmt.Sprintf("tokens: %d in, %d out", m.report.InputTokens, m.report.OutputTokens))))
}

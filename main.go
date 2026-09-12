// Observatory checks the changes in a git repository against rules in plain
// language. A model (the Anthropic API, Claude Code or GitHub Copilot) is the
// reviewer.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

const usage = `observatory — check a git repo against human-readable rules

Usage:
  observatory                         start interactive mode
  observatory init                    make the .observatory directory and its config
  observatory list                    list the rules
  observatory add <title>             make a rule and print its directory
  observatory run [scope]             check the changes against the rules
  observatory doctor                  check that everything observatory needs works
  observatory version                 show the observatory version
  observatory help                    show this help

Scope of run (default: the last commit, or the pull request in GitHub Actions):
  --commit <ref>                      the changes of one commit
  --uncommitted                       the changes not yet committed
  --base <ref>                        the changes between <ref> and the working tree
  --all                               every tracked file in the repo

Rules live in .observatory/<id>/rule.md; see README for the config.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "observatory: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return tui()
	}
	return dispatch(args[0], args[1:])
}

func dispatch(cmd string, args []string) error {
	switch cmd {
	case "init":
		return cmdInit(args)
	case "list":
		return cmdList(args)
	case "add":
		return cmdAdd(args)
	case "run":
		return cmdRun(args)
	case "doctor":
		return cmdDoctor(args)
	case "version", "-v", "--version":
		fmt.Println(version())
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	}
	return fmt.Errorf("unknown command %q — try 'help'", cmd)
}

// version reads the module version that Go recorded in the binary. Builds
// made with `go install <module>@<tag>` get the tag. Builds made from a
// source checkout get "(devel)" or the version that ldflags set.
func version() string {
	if v := versionOverride; v != "" {
		return v
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}

// The release workflow sets versionOverride with ldflags, because a
// binary that a workflow builds from a checkout has no module version.
var versionOverride string

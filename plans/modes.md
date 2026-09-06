# Plan: check modes

## Goal

A rule says what it needs to see. Some rules need only the diff. Some need
the whole file that changed. Some need the whole repository. Some need facts
about the repository as a whole, not the text of it.

## Current state

The scope of `run` picks one text for every rule: a diff (`--commit`,
`--uncommitted`, `--base`) or every tracked file (`--all`). A rule cannot ask
for more context than the scope gives.

## Design

Four modes, named for what the model reads:

| mode        | the model reads                                                   | good for                                          |
| ----------- | ----------------------------------------------------------------- | ------------------------------------------------- |
| `diff`      | the diff of the scope (today)                                     | "do not add X", style rules                       |
| `file`      | the full content of each file the scope touches, plus the diff    | rules that need the context of the file           |
| `repo`      | every tracked file (today: `--all`)                               | "each package has a README"                       |
| `aggregate` | an indirect view of the code that observatory computes: the syntax tree, the symbols, the imports, the call graph | "no more than one HTTP library", "each public function has a test", "no package imports the database layer except the repository package" |

A rule picks its mode in the front matter of `rule.md`; `diff` is the default:

```markdown
---
mode: file
---
# Each public function has a docstring
```

`run` groups the rules by mode, builds one text per mode, and sends one
request per group (or one per rule with `per_rule`). The report is the same.

The scope flags stay as they are: they say which change to look at. The mode
says how much of the repo to show around it. `--all` becomes a scope that
touches every file; with `mode: diff` it shows every file, as today.

`aggregate` needs research before a design. The idea: the model reads a
view of the code that the code itself does not show directly, such as the
abstract syntax tree, the list of symbols, the import graph, or the call
graph. Things to find out:

- Which parser. Candidates: tree-sitter through `smacker/go-tree-sitter` (many languages, needs cgo), `go/ast` (Go only, in the standard library), `universal-ctags` as an external program (many languages, symbols only), an SCIP or LSIF index (precise, needs a language server per language).
- Which views a rule needs. Start from real rules, then pick the smallest view that answers each one: a symbol list, an import list per file, a call graph, or the tree itself.
- How to render a view for the model. A raw syntax tree is large; a symbol table or an import list is small and reads well.
- How a rule names the view. One `mode: aggregate` and a `view:` key, or one mode per view.
- The token cost. An aggregate view of a large repository can exceed `max_tokens`; find the size of each candidate view on a real repository.

The first three modes do not wait for this research.

## Steps

1. Parse a `mode:` front matter key in `loadRules`; reject an unknown mode. Test it.
2. `file` mode: for each path in the diff (`git diff --name-only`), read the file at the working tree and render `==== path ====` blocks, then the diff. Reuse `allFiles` for the rendering.
3. `repo` mode: `allFiles`, independent of the scope.
4. `aggregate` mode: research first, see above. Then a design in this file, then the code.
5. Group rules by mode in `check`, one request per group; sum the token usage.
6. The prompt says what the model reads: "the full content of the files that changed" and so on.
7. README: a "Modes" section, and the sample rule `OBS-001` stays in `diff` mode.

## Open questions

- Which rules need `aggregate` first? They decide the parser and the views.
- Token budget: `file` and `repo` modes can be large. One `max_tokens` for the run, or one per mode?
- A big `file` mode diff may need a per-file split. Skip it until a repo hits the limit.

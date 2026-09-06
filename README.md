# observatory

Observatory is a command line tool and a terminal user interface. It checks a
git repository against rules that you write in plain language. A rule can be
"use httpx for every HTTP call" or "each public function has a docstring".
Observatory sends the rules and the changes to Claude, and Claude gives a
verdict for each rule. Think of it as `pytest` for the rules that a linter
cannot express.

Observatory runs on your machine, or in a GitHub Actions workflow.

## Install

```sh
make build                                                  # then move bin/observatory into a directory on your PATH
go install github.com/Open-Source-Lodge/observatory@latest
```

Observatory needs `git`, and an Anthropic API key in `ANTHROPIC_API_KEY`.

## Commands

```
observatory                         start interactive mode
observatory init                    make the .observatory directory and its config
observatory list                    list the rules
observatory add <title>             make a rule and print its directory
observatory run [scope]             check the changes against the rules
observatory doctor                  check that everything observatory needs works
observatory version                 show the observatory version
observatory help                    show the help
```

## Rules

The rules live in the repository, in the `.observatory` directory. Check the
directory in, so that the rules travel with the code. Each rule has its own
directory. The name of the directory is the ID of the rule. Observatory makes
the ID when you add the rule: `OBS-001`, then `OBS-002`, and so on.

```
.observatory/
├── config
├── OBS-001/
│   ├── rule.md
│   └── explain.md
└── OBS-002/
    ├── rule.md
    └── explain.md
```

Two branches that each add a rule get the same ID. Rename one of the
directories when you merge them.

`rule.md` is the rule. Keep it short and exact, because this file goes into
the prompt for each run. The first `# ` heading is the title of the rule.

`explain.md` is the long explanation, for people. Write the reasons for the
rule here, and the business context. Observatory does not send this file to
the model in a run.

The `examples` directory holds example rules of different types. Copy one
into `.observatory` and change it to match your repository.

### add

The `observatory add <title>` command makes the directory and the two files.
Then write the rule in `rule.md`:

```sh
observatory add Use httpx for HTTP calls
# .observatory/OBS-001
# write the rule in rule.md, and the reasons for it in explain.md
```

### run

The `observatory run` command sends the rules and the changes to the model.
It prints one line per rule, and the number of tokens the run used. The exit
code is 1 when a rule fails, so a workflow step fails too:

```
checked the commit HEAD
PASS  OBS-001  No file in the diff makes an HTTP call.
FAIL  OBS-002  src/api.py line 12 adds `print(response)`; the rule asks for the logger.
tokens: 2310 in, 96 out
```

The scope says which changes the model reads:

```sh
observatory run                   # the last commit
observatory run --commit abc123   # one commit
observatory run --uncommitted     # the changes not yet committed, staged or not
observatory run --base develop    # the changes since the branch develop, up to the working tree
observatory run --all             # every tracked file in the repository
```

For `--base`, observatory finds the point where your branch left the given
branch, and reads the changes from that point up to the working tree. In
GitHub Actions, a run without a scope uses the base branch of the pull
request in this manner.

A rule that the changes do not touch passes. Use `--all` to check the whole
repository, for example after you add a rule. The `--all` scope reads every
tracked file, so it can exceed `max_tokens` in a large repository.

### Config

The file `.observatory/config` holds the settings. The format is `key = value`
with `#` comments. The `init` command writes the defaults:

```toml
model = "claude-opus-5"
max_tokens = 200000
max_output_tokens = 8000
per_rule = false
```

| key                 | meaning                                                                           |
| ------------------- | --------------------------------------------------------------------------------- |
| `model`             | the Claude model that checks the rules                                            |
| `max_tokens`        | the largest prompt observatory sends; a run that would send more stops before it spends anything |
| `max_output_tokens` | the largest answer observatory accepts from the model                             |
| `per_rule`          | `true` sends one request per rule; `false` sends one request for all the rules   |

One request for all the rules is the cheaper option. One request per rule
gives each rule the full attention of the model, and costs a request per rule.

## Interactive mode

Start `observatory` with no arguments to see the rules:

```
  observatory · /home/me/myrepo/.observatory

❯ OBS-001  Use httpx for HTTP calls  Every HTTP call goes through httpx.
  OBS-002  No print statements       Use the logger, not print.

  ↑↓ move · n new · e editor · d delete · R run · r refresh · q quit
```

| key             | operation                                         |
| --------------- | ------------------------------------------------- |
| `↑` `↓` `k` `j` | move the selection                                |
| `n`             | make a rule: type the title, the rule and the why |
| `e` `enter`     | open the directory of the rule in your editor     |
| `d`             | delete the rule, after a confirmation             |
| `R`             | check the last commit                             |
| `r`             | read the rules again                              |
| `q` `esc`       | stop observatory                                  |

The `e` key looks for the editor in this sequence: `$OBSERVATORY_EDITOR`, then
`$VISUAL`, then `$EDITOR`. The value is a command with arguments, such as
`code -n`.

## GitHub Actions

Add the API key as a secret, then use the action from this repository:

```yaml
name: observatory
on:
  pull_request:
jobs:
  rules:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0 # observatory needs the history back to the base branch
      - uses: Open-Source-Lodge/observatory@main
        with:
          anthropic-api-key: ${{ secrets.ANTHROPIC_API_KEY }}
```

A failed rule fails the job. Observatory also writes an annotation for each
failed rule, so the failure shows on the file in the pull request.

## Development

```sh
make            # lint, test, build
make test
make lint
```

### Documentation rules

Write all documentation in this repository in ASD-STE100 Simplified Technical
English. These are the primary rules:

- Use the words from the STE dictionary. Technical names, such as `rule`,
  `commit` and `token`, are permitted.
- Give one meaning to each word. Do not use a word as a noun and as a verb.
- Use the same word for the same thing in all the documents.
- Write short sentences. Use a maximum of 20 words in an instruction, and a
  maximum of 25 words in a description.
- Write one instruction in one sentence.
- Use the active voice. Do not use the passive voice.
- Use the simple present tense, the simple past tense or the simple future
  tense.
- Do not use the `-ing` form of a verb, unless it is part of a technical name.
- Use the articles `the`, `a` and `an` where possible.
- Do not remove words to make a sentence shorter.
- Write a maximum of six sentences in a paragraph.
- Write about one topic in one paragraph.
- Do not use contractions, idioms, slang or jargon.
- Do not use words from a different language.

Text in a code block shows the output of the program. Do not change that text
in the documentation. Change the program first.

<p align="center"><img src="assets/logo.svg" alt="Observatory logo" width="520"></p>

# observatory

Observatory is a test system that uses LLMs to make sure that your
repository, code, product or service obeys your rules. It is a checker for
rules that can be hard, or even not possible, to write as code.

You write the rules in plain language. A rule can be "use httpx for every HTTP
call" or "each public function has a docstring". Observatory sends the rules
and the changes to the model, and the model gives a verdict for each rule.

Observatory has a command line tool and a terminal user interface. It runs on
your machine, or in a GitHub Actions workflow.

## Install

```sh
make build                                                  # then move bin/observatory into a directory on your PATH
go install github.com/Open-Source-Lodge/observatory@latest
```

Each [release](https://github.com/Open-Source-Lodge/observatory/releases) has
binaries for macOS, Linux and Windows, on amd64 and arm64. Download the binary
for your system, check it against `checksums.txt`, and move it into a
directory on your PATH.

Observatory needs `git`, and an Anthropic API key in `ANTHROPIC_API_KEY`. See
"Providers" for the other APIs and for the Claude Code and GitHub Copilot
commands.

## Commands

```
observatory                         start interactive mode
observatory init                    make the .observatory directory and its config
observatory list                    list the rules
observatory add <title>             make a rule and print its directory
observatory run [scope]             check the changes against the rules
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
│   ├── explain.md
│   └── ignore
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

`ignore` is optional. It lists the files that the rule does not see, one
pattern per line, with `#` comments. A pattern is a file, a directory, or a
glob:

```
docs/            # every file in the directory
*.md             # every file with this name, in every directory
src/legacy.py    # one file
**/testdata      # this name, in every directory
```

A pattern with a slash starts at the root of the repository. A pattern
without a slash matches at every depth. Observatory gives the patterns to
git, and git leaves the files that match out of the changes. A rule that
ignores every file in the changes passes without a request.

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
While the model works, it shows a spinner on stderr with the scope, the
provider, the model, the rule in progress and the elapsed time. A log without
a terminal gets one line instead. Then it prints the model, one line per rule, and
the number of tokens the run used. The exit code is 1 when a rule fails, so
a workflow step fails too:

```
model: claude-sonnet-4-5
checked the commit HEAD
PASS  OBS-001  No file in the diff makes an HTTP call.
FAIL  OBS-002  src/api.py line 12 adds `print(response)`; the rule asks for the logger.
tokens: 2310 in, 96 out
```

When `per_rule` is `true`, each rule line also shows the tokens of its own
request, as `(1150 in, 40 out)`.

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
provider = "anthropic"
model = "claude-opus-5"
base_url = ""
api_key_env = ""
max_tokens = 200000
max_output_tokens = 8000
per_rule = false
```

| key                 | meaning                                                                           |
| ------------------- | --------------------------------------------------------------------------------- |
| `provider`          | the API that checks the rules: `anthropic`, `openai`, `claude` or `copilot`       |
| `model`             | the model that checks the rules                                                   |
| `base_url`          | the URL of the API; empty is the default of the provider                          |
| `api_key_env`       | the environment variable that holds the API key; empty is `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` |
| `max_tokens`        | the largest prompt observatory sends; a run that would send more stops before it spends anything |
| `max_output_tokens` | the largest answer observatory accepts from the model                             |
| `per_rule`          | `true` sends one request per rule; `false` sends one request for all the rules   |

One request for all the rules is the cheaper option. One request per rule
gives each rule the full attention of the model, and costs a request per rule.
The rules with the same `ignore` file share a request. A rule with a
different `ignore` file gets its own request, because it sees different
changes.

An environment variable `OBSERVATORY_<KEY>` replaces the value of a key. Write
the key in upper case, for example `OBSERVATORY_MODEL=claude-sonnet-5`. The
environment variable wins over the config file.

### Providers

The `anthropic` provider is the default. It uses the Anthropic API with the
key in `ANTHROPIC_API_KEY`. If that variable is empty, it uses the OAuth token
in `ANTHROPIC_AUTH_TOKEN`.

The `openai` provider sends a chat completions request to `base_url`. OpenAI,
Ollama, OpenRouter and Groq all accept this request. The provider reads the
key from `OPENAI_API_KEY`. Ollama does not check the key, but the variable
must have a value. This config uses a local Ollama:

```toml
provider = "openai"
model = "llama3.3"
base_url = "http://localhost:11434/v1"
```

Observatory estimates the size of the prompt for the `max_tokens` check, at
four bytes per token. The usage that the provider reports after the request
gives the true count.

The `claude` provider runs the `claude` command of Claude Code. The command
uses its own login, so a Claude subscription works without an API key. Log in
once with `claude`, then set the provider:

```toml
provider = "claude"
model = "opus"
```

The provider runs `claude -p` with no tools and no MCP servers, and estimates
the tokens as the `openai` provider does. The `max_output_tokens` key has no
effect on this provider. The provider removes `ANTHROPIC_API_KEY` and
`ANTHROPIC_AUTH_TOKEN` from the environment of the command, so the command
always uses its login. Use the `anthropic` provider to check with an API key.

The `copilot` provider runs the `copilot` command of the GitHub Copilot CLI.
The command uses its own login, so a GitHub Copilot subscription works
without an API key. Install the command with `npm install -g @github/copilot`,
log in once with `copilot login`, then set the provider:

```toml
provider = "copilot"
model = "auto"
```

The `model` is a name that Copilot knows, such as `gpt-5.4` or
`claude-sonnet-4.5`. The value `auto` lets Copilot select the model. The
provider runs `copilot` with no tools and no MCP servers. The command reports
no token counts, so the `tokens` line shows an estimate. The `max_output_tokens`
key has no effect on this provider.

## Interactive mode

Start `observatory` with no arguments to see the rules:

```
  observatory · /home/me/myrepo/.observatory · scope: the last commit

❯ OBS-001  Use httpx for HTTP calls  Every HTTP call goes through httpx.
  OBS-002  No print statements       Use the logger, not print.

  ↑↓ move · n new · e editor · d delete · s scope · r run · R run all · ctrl+r refresh · q quit
```

| key             | operation                                                                         |
| --------------- | --------------------------------------------------------------------------------- |
| `↑` `↓` `k` `j` | move the selection                                                                |
| `n`             | make a rule: type the title, the rule and the why                                 |
| `ctrl+g`        | in the new rule form: the model writes the fields that are empty                  |
| `e` `enter`     | open the directory of the rule in your editor                                     |
| `d`             | delete the rule, after a confirmation                                             |
| `s`             | change the scope: the last commit, the uncommitted changes, or every tracked file |
| `r`             | check the scope against the selected rule                                         |
| `R`             | check the scope against every rule                                                |
| `ctrl+r`        | read the rules again                                                              |
| `q` `esc`       | stop observatory                                                                  |

The `e` key looks for the editor in this sequence: `$OBSERVATORY_EDITOR`, then
`$VISUAL`, then `$EDITOR`. The value is a command with arguments, such as
`code -n`.

In the new rule form, type the title, or the rule, or the why, then press
`ctrl+g`. The model of the config writes the fields that are empty. Change the
text, then press `enter` to make the rule.

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
      - uses: Open-Source-Lodge/observatory@v1
        with:
          anthropic-api-key: ${{ secrets.ANTHROPIC_API_KEY }}
```

A failed rule fails the job. Observatory also writes an annotation for each
failed rule, so the failure shows on the file in the pull request.

The `copilot` provider has two ways to log in from GitHub Actions. A
fine-grained personal access token with the "Copilot Requests" permission,
in the secret `COPILOT_GITHUB_TOKEN`, uses the Copilot subscription of its
owner. The token of the workflow, `GITHUB_TOKEN`, uses the Copilot Business
subscription of the organization instead. For `GITHUB_TOKEN`, the organization
must permit the Copilot CLI to use it. See the [GitHub documentation](https://docs.github.com/en/copilot/how-tos/copilot-cli/use-copilot-cli-in-actions)
for the policy. Install the `copilot` command in a step before the action:

```yaml
name: observatory
on:
  pull_request:
permissions:
  contents: read
  copilot-requests: write
jobs:
  rules:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: npm install -g @github/copilot
      - uses: Open-Source-Lodge/observatory@v1
        env:
          COPILOT_GITHUB_TOKEN: ${{ secrets.COPILOT_GITHUB_TOKEN }} # or GITHUB_TOKEN: ${{ github.token }}
          OBSERVATORY_PROVIDER: copilot
          OBSERVATORY_MODEL: auto
```

This repository checks its own rules with this provider. See
`.github/workflows/observatory.yml`.

## Development

```sh
make            # lint, test, build
make test
make lint
```

### Documentation rules

Write all documentation in this repository in ASD-STE100 Simplified Technical
English. These are the primary rules:

- Use the words from the STE dictionary. You can use technical names, such as
  `rule`, `commit` and `token`.
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

## License

Copyright (C) 2026 Stephan Nordnes Eriksen. Observatory is free software
under the GNU AGPLv3. See `LICENSE`.

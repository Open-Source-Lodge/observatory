# Plan: multiple LLM providers

## Goal

A repo can pick the provider that checks its rules. Anthropic stays the
default. An OpenAI-compatible HTTP endpoint covers OpenAI, Ollama, OpenRouter,
Groq and most others with one implementation.

## Current state

`run.go` calls the Anthropic Go SDK directly in `ask()`: one `CountTokens`
call for the budget, then one `Messages.New` call with a JSON schema.

## Design

Config keys in `.observatory/config`:

```toml
provider = "anthropic"          # or "openai"
model = "claude-opus-5"
base_url = ""                   # openai only; default https://api.openai.com/v1, Ollama is http://localhost:11434/v1
api_key_env = "ANTHROPIC_API_KEY"  # the environment variable that holds the key
```

One small interface in `run.go`, with two implementations:

```go
type provider interface {
    // countTokens is the size of the prompt, for the max_tokens check.
    countTokens(ctx context.Context, prompt string) (int64, error)
    // complete returns the JSON answer and the tokens the request used.
    complete(ctx context.Context, prompt string, maxOut int) (text string, in, out int64, err error)
}
```

- `anthropicProvider`: the code that is in `ask()` today.
- `openaiProvider`: `net/http` and `encoding/json` against `POST {base_url}/chat/completions`
  with `response_format: {type: "json_schema", ...}`. No new dependency.
  `countTokens` has no endpoint on this API: estimate `len(prompt)/4`, and
  mark it with a `ponytail:` comment. The usage in the response gives the
  true count after the fact.

`ask()` keeps the prompt, the schema and the parse. Only the two calls move
behind the interface. `doctor` reports the provider and checks `api_key_env`.

## Steps

1. Move the two SDK calls in `ask()` into `anthropicProvider`. No behavior change; tests still pass.
2. Add the config keys and `newProvider(cfg)`; unknown provider is an error at load time.
3. Add `openaiProvider` with an `httptest` test for the request body and the parse of the answer.
4. `doctor`: show the provider, check the key, and try a one-token request when `--online` is given.
5. README: a "Providers" section with the Ollama example.

## Open questions

- Is a native OpenAI SDK worth a dependency over raw HTTP? Raw HTTP is about 60 lines.
- Gemini and Bedrock: only on request. Bedrock has an Anthropic SDK client; add a `provider = "bedrock"` that reuses `anthropicProvider` with a different client.

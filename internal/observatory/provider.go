package observatory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// provider is the model API that answers a prompt.
type provider interface {
	// complete returns the JSON answer and the tokens the request used. The
	// schema is the shape of the answer; a provider without a schema option
	// ignores it.
	complete(ctx context.Context, prompt string, schema map[string]any) (text string, in, out int64, err error)
}

// newProvider makes the provider that the config names. It is a variable,
// so that a test can replace the real provider.
var newProvider = func(cfg Config) (provider, error) {
	switch cfg.Provider {
	case "anthropic":
		return anthropicProvider{cfg: cfg}, nil
	case "openai":
		return openaiProvider{cfg: cfg}, nil
	case "claude":
		if _, err := exec.LookPath("claude"); err != nil {
			return nil, errors.New("the claude command is not installed: see https://claude.com/claude-code")
		}
		return claudeProvider{cfg: cfg}, nil
	case "copilot":
		if _, err := exec.LookPath("copilot"); err != nil {
			return nil, errors.New("the copilot command is not installed: run npm install -g @github/copilot")
		}
		return copilotProvider{cfg: cfg}, nil
	}
	return nil, fmt.Errorf("unknown provider %q: use \"anthropic\", \"openai\", \"claude\" or \"copilot\"", cfg.Provider)
}

// postJSON sends body to url as JSON and decodes the answer into out.
//
// postJSON does not retry after a 429 or a 5xx Status. Add a retry when a
// run fails on one.
func postJSON(ctx context.Context, url string, headers map[string]string, body, out any) error {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unexpected answer from %s: %s", url, strings.TrimSpace(string(data)))
	}
	return nil
}

// estimate is the size of a text in tokens. Four bytes per token is the
// usual ratio. The estimate guards max_tokens before the request; the usage
// in the answer gives the true count.
func estimate(text string) int64 {
	return int64(len(text) / 4)
}

// userMessage is the one message of a request: the whole prompt.
func userMessage(prompt string) []map[string]string {
	return []map[string]string{{"role": "user", "content": prompt}}
}

// anthropicProvider uses the Messages API of Anthropic.
type anthropicProvider struct {
	cfg Config
}

// headers has the API version and the credentials. The key goes in
// x-api-key. An OAuth token in ANTHROPIC_AUTH_TOKEN goes in Authorization.
func (p anthropicProvider) headers() map[string]string {
	h := map[string]string{"anthropic-version": "2023-06-01"}
	if key := os.Getenv(p.cfg.APIKeyEnv); key != "" {
		h["x-api-key"] = key
	} else if token := os.Getenv("ANTHROPIC_AUTH_TOKEN"); token != "" {
		h["Authorization"] = "Bearer " + token
	}
	return h
}

func (p anthropicProvider) complete(ctx context.Context, prompt string, schema map[string]any) (string, int64, int64, error) {
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason  string `json:"stop_reason"`
		StopDetails struct {
			Explanation string `json:"explanation"`
		} `json:"stop_details"`
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	err := postJSON(ctx, strings.TrimRight(p.cfg.BaseURL, "/")+"/v1/messages", p.headers(), map[string]any{
		"model":      p.cfg.Model,
		"max_tokens": p.cfg.MaxOutputTokens,
		"messages":   userMessage(prompt),
		"output_config": map[string]any{
			"format": map[string]any{"type": "json_schema", "schema": schema},
		},
	}, &out)
	if err != nil {
		return "", 0, 0, err
	}
	in, used := out.Usage.InputTokens, out.Usage.OutputTokens
	switch out.StopReason {
	case "refusal":
		return "", in, used, errors.New("the model declined the request: " + out.StopDetails.Explanation)
	case "max_tokens":
		return "", in, used, errMaxOutput(p.cfg)
	}
	var text strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	return text.String(), in, used, nil
}

// openaiProvider uses a chat completions endpoint. OpenAI, Ollama, OpenRouter
// and most other APIs have one.
type openaiProvider struct {
	cfg Config
}

func (p openaiProvider) complete(ctx context.Context, prompt string, schema map[string]any) (string, int64, int64, error) {
	headers := map[string]string{}
	if key := os.Getenv(p.cfg.APIKeyEnv); key != "" {
		headers["Authorization"] = "Bearer " + key
	}
	var out struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
				Refusal string `json:"refusal"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	err := postJSON(ctx, strings.TrimRight(p.cfg.BaseURL, "/")+"/chat/completions", headers, map[string]any{
		"model": p.cfg.Model,
		// The key max_tokens works on Ollama, OpenRouter and Groq. The OpenAI
		// reasoning models need max_completion_tokens.
		"max_tokens": p.cfg.MaxOutputTokens,
		"messages":   userMessage(prompt),
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "answer",
				"strict": true,
				"schema": schema,
			},
		},
	}, &out)
	if err != nil {
		return "", 0, 0, err
	}
	if len(out.Choices) == 0 {
		return "", 0, 0, errors.New("the answer has no choices")
	}
	in, used := out.Usage.PromptTokens, out.Usage.CompletionTokens
	c := out.Choices[0]
	if c.Message.Refusal != "" {
		return "", in, used, errors.New("the model declined the request: " + c.Message.Refusal)
	}
	if c.FinishReason == "length" {
		return "", in, used, errMaxOutput(p.cfg)
	}
	return c.Message.Content, in, used, nil
}

// claudeProvider runs the Claude Code command. It uses the login of the
// command, so a Claude subscription works without an API key.
type claudeProvider struct {
	cfg Config
}

func (p claudeProvider) complete(ctx context.Context, prompt string, schema map[string]any) (string, int64, int64, error) {
	schemaJSON, _ := json.Marshal(schema)
	// The prompt goes on stdin: a large diff does not fit in an argument.
	// No tools and no MCP servers: the model must judge only the prompt.
	cmd := exec.CommandContext(ctx, "claude", "-p", "--output-format", "json",
		"--model", p.cfg.Model, "--json-schema", string(schemaJSON),
		"--tools", "", "--strict-mcp-config")
	cmd.Stdin = strings.NewReader(prompt)
	// The command must use its own login. An API key in the environment
	// takes precedence over the login, and a key the API rejects makes the
	// command retry for minutes, which looks like a hang.
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") || strings.HasPrefix(kv, "ANTHROPIC_AUTH_TOKEN=")
	})
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return "", 0, 0, fmt.Errorf("claude: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var out struct {
		Subtype          string          `json:"subtype"`
		IsError          bool            `json:"is_error"`
		Result           string          `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
		Usage            struct {
			InputTokens         int64 `json:"input_tokens"`
			OutputTokens        int64 `json:"output_tokens"`
			CacheReadTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", 0, 0, fmt.Errorf("unexpected answer from claude: %s", strings.TrimSpace(string(data)))
	}
	in := out.Usage.InputTokens + out.Usage.CacheReadTokens + out.Usage.CacheCreationTokens
	if out.IsError || out.Subtype != "success" {
		return "", in, out.Usage.OutputTokens, fmt.Errorf("claude: %s: %s", out.Subtype, out.Result)
	}
	if len(out.StructuredOutput) > 0 {
		return string(out.StructuredOutput), in, out.Usage.OutputTokens, nil
	}
	return out.Result, in, out.Usage.OutputTokens, nil
}

// copilotProvider runs the GitHub Copilot CLI command. It uses the login of
// the command, or GITHUB_TOKEN in GitHub Actions.
type copilotProvider struct {
	cfg Config
}

func (p copilotProvider) complete(ctx context.Context, prompt string, _ map[string]any) (string, int64, int64, error) {
	// The prompt goes on stdin: the command reads stdin as the prompt when
	// stdin is not a terminal. No tools and no MCP servers: the model must
	// judge only the prompt.
	// The flag --available-tools with an empty value permits every tool.
	// Thus the value names a tool that does not exist.
	cmd := exec.CommandContext(ctx, "copilot", "--silent", "--model", p.cfg.Model,
		"--available-tools=none", "--disable-builtin-mcps", "--no-custom-instructions",
		"--no-ask-user", "--no-auto-update", "--no-color")
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return "", 0, 0, fmt.Errorf("copilot: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	// The command does not report the token usage. The two counts are
	// estimates.
	return string(data), estimate(prompt), estimate(string(data)), nil
}

func errMaxOutput(cfg Config) error {
	return fmt.Errorf("the answer hit max_output_tokens (%d) — raise it in %s/config", cfg.MaxOutputTokens, rulesDirName)
}

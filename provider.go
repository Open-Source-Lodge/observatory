package main

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
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// provider is the model API that answers a prompt.
type provider interface {
	// countTokens is the size of the prompt, for the max_tokens check.
	countTokens(ctx context.Context, prompt string) (int64, error)
	// complete returns the JSON answer and the tokens the request used.
	complete(ctx context.Context, prompt string) (text string, in, out int64, err error)
}

// newProvider makes the provider that the config names.
func newProvider(cfg Config) (provider, error) {
	switch cfg.Provider {
	case "anthropic":
		var opts []option.RequestOption
		if key := os.Getenv(cfg.APIKeyEnv); key != "" {
			opts = append(opts, option.WithAPIKey(key))
		}
		if cfg.BaseURL != "" {
			opts = append(opts, option.WithBaseURL(cfg.BaseURL))
		}
		return anthropicProvider{cfg: cfg, client: anthropic.NewClient(opts...)}, nil
	case "openai":
		return openaiProvider{cfg: cfg}, nil
	case "claude":
		if _, err := exec.LookPath("claude"); err != nil {
			return nil, errors.New("the claude command is not installed: see https://claude.com/claude-code")
		}
		return claudeProvider{cfg: cfg}, nil
	}
	return nil, fmt.Errorf("unknown provider %q: use \"anthropic\", \"openai\" or \"claude\"", cfg.Provider)
}

// anthropicProvider uses the Anthropic API through its Go SDK.
type anthropicProvider struct {
	cfg    Config
	client anthropic.Client
}

func (p anthropicProvider) countTokens(ctx context.Context, prompt string) (int64, error) {
	count, err := p.client.Messages.CountTokens(ctx, anthropic.MessageCountTokensParams{
		Model:    anthropic.Model(p.cfg.Model),
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
	})
	if err != nil {
		return 0, err
	}
	return count.InputTokens, nil
}

func (p anthropicProvider) complete(ctx context.Context, prompt string) (string, int64, int64, error) {
	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.cfg.Model),
		MaxTokens: int64(p.cfg.MaxOutputTokens),
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: findingsSchema()},
		},
	})
	if err != nil {
		return "", 0, 0, err
	}
	in, out := resp.Usage.InputTokens, resp.Usage.OutputTokens
	switch resp.StopReason {
	case anthropic.StopReasonRefusal:
		return "", in, out, errors.New("the model declined the request: " + resp.StopDetails.Explanation)
	case anthropic.StopReasonMaxTokens:
		return "", in, out, errMaxOutput(p.cfg)
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	return text.String(), in, out, nil
}

// openaiProvider uses a chat completions endpoint: OpenAI, Ollama, OpenRouter
// and most others speak it.
type openaiProvider struct {
	cfg Config
}

func (p openaiProvider) countTokens(_ context.Context, prompt string) (int64, error) {
	// ponytail: this API has no count endpoint. Four bytes per token is the
	// usual estimate; the usage in the answer gives the true count.
	return int64(len(prompt) / 4), nil
}

func (p openaiProvider) complete(ctx context.Context, prompt string) (string, int64, int64, error) {
	body, _ := json.Marshal(map[string]any{
		"model": p.cfg.Model,
		// ponytail: max_tokens works on Ollama, OpenRouter and Groq. The
		// OpenAI reasoning models want max_completion_tokens instead.
		"max_tokens": p.cfg.MaxOutputTokens,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "findings",
				"strict": true,
				"schema": findingsSchema(),
			},
		},
	})
	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key := os.Getenv(p.cfg.APIKeyEnv); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, 0, err
	}
	if resp.StatusCode/100 != 2 {
		return "", 0, 0, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
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
	if err := json.Unmarshal(data, &out); err != nil || len(out.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("unexpected answer from %s: %s", url, strings.TrimSpace(string(data)))
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

func (p claudeProvider) countTokens(_ context.Context, prompt string) (int64, error) {
	// ponytail: the count endpoint needs an API key. Estimate as openai does.
	return int64(len(prompt) / 4), nil
}

func (p claudeProvider) complete(ctx context.Context, prompt string) (string, int64, int64, error) {
	schema, _ := json.Marshal(findingsSchema())
	// The prompt goes on stdin: a large diff does not fit in an argument.
	// No tools and no MCP servers: the model must judge only the prompt.
	cmd := exec.CommandContext(ctx, "claude", "-p", "--output-format", "json",
		"--model", p.cfg.Model, "--json-schema", string(schema),
		"--tools", "", "--strict-mcp-config",
		"--system-prompt", "You review changes to a code repository against the rules of that repository.")
	cmd.Stdin = strings.NewReader(prompt)
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

func errMaxOutput(cfg Config) error {
	return fmt.Errorf("the answer hit max_output_tokens (%d) — raise it in %s/config", cfg.MaxOutputTokens, rulesDirName)
}

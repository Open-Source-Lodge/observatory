package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// rulesDirName is the directory in the repo that holds the rules and the config.
const rulesDirName = ".observatory"

// Config is the content of `.observatory/config`. Every key has a default,
// so a repo without a config file still runs.
type Config struct {
	// Provider is the API that checks the rules: "anthropic", "openai",
	// "claude" for the Claude Code command and its login, or "copilot" for
	// the GitHub Copilot CLI command and its login.
	Provider string
	// Model is the model that checks the rules.
	Model string
	// BaseURL replaces the default URL of the provider. Empty is the default.
	BaseURL string
	// APIKeyEnv is the environment variable that holds the API key.
	APIKeyEnv string
	// MaxTokens is the largest prompt observatory sends. A run whose rules
	// and changes exceed it stops before it spends anything.
	MaxTokens int
	// MaxOutputTokens caps the answer of the model.
	MaxOutputTokens int
	// PerRule sends one request per rule instead of one request for all
	// rules. It costs more, and gives each rule the full attention of the model.
	PerRule bool
}

func defaultConfig() Config {
	return Config{Provider: "anthropic", Model: "claude-opus-5", BaseURL: "https://api.anthropic.com", APIKeyEnv: "ANTHROPIC_API_KEY", MaxTokens: 200000, MaxOutputTokens: 8000}
}

// configTemplate is what `init` writes. Keep it in sync with defaultConfig.
const configTemplate = `# observatory settings for this repo — check this file in.
# An environment variable OBSERVATORY_<KEY> in upper case replaces a key, for example OBSERVATORY_MODEL.

# The API that checks the rules: "anthropic", "openai" for every chat completions endpoint,
# "claude" for the Claude Code command and the login of your Claude subscription,
# or "copilot" for the GitHub Copilot CLI command and the login of your GitHub account.
provider = "anthropic"

# The model that checks the rules.
model = "claude-opus-5"

# The URL of the API. Empty is the default of the provider. Ollama is http://localhost:11434/v1
base_url = ""

# The environment variable that holds the API key. Empty is ANTHROPIC_API_KEY or OPENAI_API_KEY.
api_key_env = ""

# The largest prompt observatory sends, in tokens. A run that would send more stops.
max_tokens = 200000

# The largest answer observatory accepts from the model, in tokens.
max_output_tokens = 8000

# true: one request per rule. false: one request for all the rules.
per_rule = false
`

// loadConfig reads `<dir>/config`. A missing file gives the defaults. An
// environment variable `OBSERVATORY_<KEY>` replaces the value of a key.
func loadConfig(dir string) Config {
	cfg := defaultConfig()
	path := filepath.Join(dir, "config")
	value := func(key string) string {
		if v := os.Getenv("OBSERVATORY_" + strings.ToUpper(key)); v != "" {
			return v
		}
		return fileValue(path, key)
	}
	if v := value("provider"); v != "" {
		cfg.Provider = v
	}
	if v := value("model"); v != "" {
		cfg.Model = v
	}
	if v := value("base_url"); v != "" {
		cfg.BaseURL = v
	} else if cfg.Provider == "openai" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if v := value("api_key_env"); v != "" {
		cfg.APIKeyEnv = v
	} else if cfg.Provider == "openai" {
		cfg.APIKeyEnv = "OPENAI_API_KEY"
	}
	if n, err := strconv.Atoi(value("max_tokens")); err == nil && n > 0 {
		cfg.MaxTokens = n
	}
	if n, err := strconv.Atoi(value("max_output_tokens")); err == nil && n > 0 {
		cfg.MaxOutputTokens = n
	}
	if on, err := strconv.ParseBool(value("per_rule")); err == nil {
		cfg.PerRule = on
	}
	return cfg
}

// rulesDir is the `.observatory` directory of the repo the command runs in.
func rulesDir() (string, error) {
	root, err := git("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Join(root, rulesDirName), nil
}

// editorCommand is the editor to open a rule with, as command and arguments.
// It is, in order of precedence: $OBSERVATORY_EDITOR, $VISUAL, $EDITOR.
func editorCommand() []string {
	for _, v := range []string{
		os.Getenv("OBSERVATORY_EDITOR"),
		os.Getenv("VISUAL"),
		os.Getenv("EDITOR"),
	} {
		if fields := strings.Fields(v); len(fields) > 0 {
			return fields
		}
	}
	return nil
}

// fileValue reads key from a config file. The format is a minimal subset of
// TOML: `key = value` lines with `#` comments.
func fileValue(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		k, val, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(k) != key {
			continue
		}
		return strings.Trim(strings.TrimSpace(val), `"'`)
	}
	return ""
}

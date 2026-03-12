package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type FormatConfig struct {
	DateFormat  string   `toml:"date_format"`
	Frontmatter []string `toml:"frontmatter"`
	ListStyle   string   `toml:"list_style"`
	Naming      string   `toml:"naming"`
	SlugStyle   string   `toml:"slug_style"`
}

type ModelConfig struct {
	Light string `toml:"light"` // low-reasoning tasks: format, frontmatter (default: haiku)
	Heavy string `toml:"heavy"` // high-reasoning tasks: ask, query (default: sonnet)
}

type BotConfig struct {
	Name          string `toml:"name"`
	Personality   string `toml:"personality"`
	AskReadBudget int    `toml:"ask_read_budget"`
}

type Config struct {
	NotesDir string       `toml:"notes_dir"`
	Debug    bool         `toml:"debug"`
	Format   FormatConfig `toml:"format"`
	Models   ModelConfig  `toml:"models"`
	Bot      BotConfig    `toml:"bot"`
}

// ModelAliases maps friendly names to full model IDs.
var ModelAliases = map[string]string{
	"haiku":  "claude-haiku-4-5",
	"sonnet": "claude-sonnet-4-5",
	"opus":   "claude-opus-4-5",
}

func DefaultConfig() Config {
	return Config{
		NotesDir: "~/notes",
		Format: FormatConfig{
			DateFormat:  "2006-01-02",
			Frontmatter: []string{"title", "date", "tags"},
			ListStyle:   "bullet",
			Naming:      "{date}-{slug}",
			SlugStyle:   "kebab",
		},
		Models: ModelConfig{
			Light: "haiku",
			Heavy: "sonnet",
		},
		Bot: BotConfig{
			AskReadBudget: 500,
		},
	}
}

// ResolveModel takes a friendly name or full model ID and returns the full ID.
func ResolveModel(name string) string {
	if id, ok := ModelAliases[name]; ok {
		return id
	}
	return name
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "nora"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func PromptsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "prompts"), nil
}

func LogsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs"), nil
}

func Load() (Config, error) {
	var cfg Config
	path, err := Path()
	if err != nil {
		return cfg, err
	}
	meta, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("could not load config at %s: %w", path, err)
	}
	if unknown := meta.Undecoded(); len(unknown) > 0 {
		for _, key := range unknown {
			fmt.Fprintf(os.Stderr, "warning: unknown config field %q (ignored)\n", key.String())
		}
	}
	if cfg.NotesDir == "" {
		return cfg, fmt.Errorf("notes_dir not set in %s", path)
	}

	// Merge with defaults for missing format fields
	defaults := DefaultConfig()
	if cfg.Format.DateFormat == "" {
		cfg.Format.DateFormat = defaults.Format.DateFormat
	}
	if len(cfg.Format.Frontmatter) == 0 {
		cfg.Format.Frontmatter = defaults.Format.Frontmatter
	}
	if cfg.Format.ListStyle == "" {
		cfg.Format.ListStyle = defaults.Format.ListStyle
	}
	if cfg.Format.Naming == "" {
		cfg.Format.Naming = defaults.Format.Naming
	}
	if cfg.Format.SlugStyle == "" {
		cfg.Format.SlugStyle = defaults.Format.SlugStyle
	}
	if cfg.Models.Light == "" {
		cfg.Models.Light = defaults.Models.Light
	}
	if cfg.Models.Heavy == "" {
		cfg.Models.Heavy = defaults.Models.Heavy
	}
	if cfg.Bot.AskReadBudget <= 0 {
		cfg.Bot.AskReadBudget = defaults.Bot.AskReadBudget
	}

	return cfg, nil
}

func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not write config: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// LoadPrompt reads a prompt file from ~/.config/nora/prompts/<name>.md.
// If the file doesn't exist, it writes the default and returns it.
func LoadPrompt(name, defaultContent string) (string, error) {
	promptsDir, err := PromptsDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(promptsDir, name+".md")

	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), nil
	}

	if !os.IsNotExist(err) {
		return "", fmt.Errorf("could not read prompt %s: %w", path, err)
	}

	// Write default
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return "", fmt.Errorf("could not create prompts directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(defaultContent), 0644); err != nil {
		return "", fmt.Errorf("could not write default prompt: %w", err)
	}

	return defaultContent, nil
}

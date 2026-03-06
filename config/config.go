package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	NotesDir string `toml:"notes_dir"`
}

func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "nnotes.toml")
}

func Load() (Config, error) {
	var cfg Config
	_, err := toml.DecodeFile(Path(), &cfg)
	if err != nil {
		return cfg, fmt.Errorf("could not load config at %s: %w", Path(), err)
	}
	if cfg.NotesDir == "" {
		return cfg, fmt.Errorf("notes_dir not set in %s", Path())
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path := Path()
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

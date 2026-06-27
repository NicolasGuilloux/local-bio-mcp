// Package config persists CLI state (auth token and selected store) on disk.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config is the on-disk state.
type Config struct {
	Token   string `json:"token,omitempty"`
	Email   string `json:"email,omitempty"`
	StoreID string `json:"storeId,omitempty"`
	BaseURL string `json:"baseUrl,omitempty"`
	App     string `json:"app,omitempty"`
}

// Dir returns the config directory, honouring LOCALBIO_CONFIG_DIR then XDG.
func Dir() (string, error) {
	if d := os.Getenv("LOCALBIO_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "local-bio"), nil
}

func path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// Load reads the config, returning an empty Config when none exists.
func Load() (*Config, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the config atomically with 0600 perms (it holds a token).
func (c *Config) Save() error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Clear removes stored credentials (keeps store/base settings).
func (c *Config) Clear() {
	c.Token = ""
	c.Email = ""
}

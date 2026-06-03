package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Subscriptions map[string]float64 `json:"subscriptions"`
}

func ConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "arc", "ai.json"), nil
}

func ReadConfig() (Config, bool, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil
		}
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	clean := map[string]float64{}
	for provider, cost := range cfg.Subscriptions {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider != "" && cost > 0 {
			clean[provider] = cost
		}
	}
	cfg.Subscriptions = clean
	return cfg, true, nil
}

func (c Config) HasSubscriptions() bool {
	return len(c.Subscriptions) > 0
}

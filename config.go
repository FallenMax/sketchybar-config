package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Colors drive every state-dependent color of the bar. App specific colors are
// configured per app (see AppConfig.Color).
type Colors struct {
	Active   string `json:"active"`
	Inactive string `json:"inactive"`
}

type AppConfig struct {
	ID    string `json:"id"`
	Icon  string `json:"icon"`
	Color string `json:"color,omitempty"`
}

type Config struct {
	Colors             Colors      `json:"colors"`
	MaxWindowsPerSpace int         `json:"maxWindowsPerSpace"`
	Apps               []AppConfig `json:"apps"`
}

func installDir() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/sketchybar"
}

func configPath() string { return installDir() + "/config.json" }

func defaultConfig() Config {
	return Config{
		Colors:             Colors{Active: "0xff60a5fa", Inactive: "0xff9ca3af"},
		MaxWindowsPerSpace: 6,
	}
}

func loadConfig() Config {
	cfg := defaultConfig()

	data, err := os.ReadFile(configPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: cannot read %s: %v\n", configPath(), err)
		return cfg
	}
	// Unmarshal over the defaults so partial configs stay usable.
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config: invalid %s: %v\n", configPath(), err)
		return defaultConfig()
	}
	if cfg.MaxWindowsPerSpace <= 0 {
		cfg.MaxWindowsPerSpace = 6
	}
	if cfg.MaxWindowsPerSpace > 20 {
		cfg.MaxWindowsPerSpace = 20
	}
	if cfg.Colors.Active == "" {
		cfg.Colors.Active = "0xff60a5fa"
	}
	if cfg.Colors.Inactive == "" {
		cfg.Colors.Inactive = "0xff9ca3af"
	}
	for i := range cfg.Apps {
		cfg.Apps[i].Icon = parseIcon(cfg.Apps[i].Icon)
	}
	return cfg
}

// parseIcon converts "U+F02AF" hex notation to the actual unicode rune.
// Raw unicode characters are passed through as-is.
func parseIcon(s string) string {
	if strings.HasPrefix(s, "U+") || strings.HasPrefix(s, "u+") {
		if code, err := strconv.ParseInt(s[2:], 16, 32); err == nil {
			return string(rune(code))
		}
	}
	return s
}

// findApp matches a window against the config by .app bundle name, with a
// fallback to yabai's display name. A "*" entry acts as catch-all so that
// unlisted apps still get an icon instead of silently disappearing.
func findApp(cfg *Config, yabaiAppName string, bundleName string) *AppConfig {
	var wildcard *AppConfig
	for i := range cfg.Apps {
		id := cfg.Apps[i].ID
		if id == "*" {
			wildcard = &cfg.Apps[i]
			continue
		}
		if id == bundleName || id == yabaiAppName {
			return &cfg.Apps[i]
		}
	}
	return wildcard
}

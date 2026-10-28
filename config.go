package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Colors are two independent knobs:
//
//	border: the pill border, the space number and the overflow counter
//	icon:   every window icon, either one color or "multicolor" to use the
//	        per-app colors from apps[].color
//
// Inactive spaces simply dim both, so a single color still gives a consistent
// look while the active space stands out.
type Colors struct {
	Border string `json:"border"`
	Icon   string `json:"icon"`
}

// multicolor tells the renderer to take the icon color from the app list.
const multicolor = "multicolor"

const (
	defaultColor = "0xff60a5fa"
	// alpha applied to border, number and icons of spaces that are not active
	dimAlpha = 0x88
)

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
		Colors:             Colors{Border: defaultColor, Icon: defaultColor},
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
	cfg.Colors.Border = normalizeColor(cfg.Colors.Border, defaultColor)
	cfg.Colors.Icon = normalizeColor(cfg.Colors.Icon, defaultColor)
	for i := range cfg.Apps {
		cfg.Apps[i].Icon = parseIcon(cfg.Apps[i].Icon)
		if cfg.Apps[i].Color != "" {
			cfg.Apps[i].Color = normalizeColor(cfg.Apps[i].Color, cfg.Colors.Icon)
		}
	}
	return cfg
}

// normalizeColor keeps valid 0xAARRGGBB values and replaces everything else
// with the fallback, so a typo cannot make the bar invisible.
func normalizeColor(value, fallback string) string {
	if value == multicolor {
		return multicolor
	}
	if _, ok := parseColor(value); ok {
		return value
	}
	return fallback
}

func parseColor(value string) (uint32, bool) {
	hex := strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
	if len(hex) != 8 {
		return 0, false
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

// withAlpha replaces the alpha channel, used to dim inactive spaces.
func withAlpha(value string, alpha uint8) string {
	v, ok := parseColor(value)
	if !ok {
		return value
	}
	v = (v & 0x00ffffff) | uint32(alpha)<<24
	return fmt.Sprintf("0x%08x", v)
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

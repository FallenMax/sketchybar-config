package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// DisplayInfo describes the screen the bar is drawn on. It is cached because
// probing it costs a swift process spawn.
type DisplayInfo struct {
	Builtin       bool `json:"builtin"`
	MenubarHeight int  `json:"menubarHeight"`
}

func cacheDir() string {
	home, _ := os.UserHomeDir()
	return home + "/.cache/sketchybar-config"
}

func displayCachePath() string { return filepath.Join(cacheDir(), "display.json") }

const defaultMenubarHeight = 24

// swiftDisplayProbe asks AppKit for both values in a single process:
// CGDisplayIsBuiltin tells us whether the main display is the internal one,
// NSStatusBar.system.thickness is the real height of the macOS menu bar.
const swiftDisplayProbe = `
import AppKit
let builtin = CGDisplayIsBuiltin(CGMainDisplayID()) != 0 ? 1 : 0
print("\(builtin) \(Int(NSStatusBar.system.thickness.rounded()))")
`

func probeDisplay() DisplayInfo {
	info := DisplayInfo{Builtin: false, MenubarHeight: defaultMenubarHeight}
	out, err := exec.Command("swift", "-e", swiftDisplayProbe).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "display: probe failed, assuming external display: %v\n", err)
		return info
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return info
	}
	info.Builtin = fields[0] == "1"
	if h, err := strconv.Atoi(fields[1]); err == nil && h >= 16 && h <= 64 {
		info.MenubarHeight = h
	}
	return info
}

func writeDisplayCache(info DisplayInfo) {
	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(info)
	if err != nil {
		return
	}
	os.WriteFile(displayCachePath(), data, 0o644)
}

func loadDisplayInfo() DisplayInfo {
	data, err := os.ReadFile(displayCachePath())
	if err == nil {
		var info DisplayInfo
		if json.Unmarshal(data, &info) == nil && info.MenubarHeight > 0 {
			return info
		}
	}
	info := probeDisplay()
	writeDisplayCache(info)
	return info
}

func detectAndCacheDisplay() DisplayInfo {
	info := probeDisplay()
	writeDisplayCache(info)
	return info
}

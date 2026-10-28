package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The install touches exactly four kinds of places on the user's system:
//   - ~/.config/sketchybar/  (binary, sketchybarrc, yabai-hook.sh, config.json)
//   - ~/.yabairc             (two appended lines)
//   - yabai signals          (in-memory, restored on every yabai start)
//   - ~/.cache/sketchybar-config/  (display probe + slot state)
// Everything printed by `summary` and `doctor` refers to this list.

const (
	hookMarker = "# sketchybar-config: keep this line so the bar reconnects after yabai restarts"
	hookLine   = `. "$HOME/.config/sketchybar/yabai-hook.sh"`

	// lines written by earlier releases, dropped on install
	legacyHookMarker   = "# sketchybar-config: auto-registered signals"
	legacyHookLineTail = "/update_sketchybar setup &"
)

func yabaircPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.yabairc"
}

func binaryPath() string { return filepath.Join(installDir(), "update_sketchybar") }

func summary() {
	fmt.Println("sketchybar-config touches exactly these places:")
	fmt.Println()
	fmt.Println("  replaced on every install")
	fmt.Println("    ~/.config/sketchybar/update_sketchybar   the bar renderer")
	fmt.Println("    ~/.config/sketchybar/sketchybarrc        entry point, run by sketchybar")
	fmt.Println("    ~/.config/sketchybar/yabai-hook.sh       reconnect logic for yabai restarts")
	fmt.Println("  created once, never overwritten")
	fmt.Println("    ~/.config/sketchybar/config.json         your app icon list")
	fmt.Println("  appended once to your yabai config")
	fmt.Println("    ~/.yabairc")
	fmt.Printf("      %s\n", hookMarker)
	fmt.Printf("      %s\n", hookLine)
	fmt.Println("  runtime only")
	fmt.Println("    yabai signals                            in-memory, restored on every yabai start")
	fmt.Println("    ~/.cache/sketchybar-config/              display probe + window slot state")
	fmt.Println()
	fmt.Println("  `make uninstall` reverts all of the above except config.json and *.bak.* backups.")
}

func setup() {
	info := detectAndCacheDisplay()
	fmt.Printf("display: builtin=%v menubar=%dpx\n", info.Builtin, info.MenubarHeight)

	if err := registerSignals(binaryPath()); err != nil {
		fmt.Fprintf(os.Stderr, "signals: %v\n", err)
		fmt.Fprintln(os.Stderr, "signals: is yabai running with its scripting addition loaded? `yabai -m query --spaces` must work.")
	} else {
		fmt.Printf("signals: %d yabai signals registered\n", len(signalsFor("")))
	}

	if err := ensureYabaircHook(); err != nil {
		fmt.Fprintf(os.Stderr, "hook: %v\n", err)
	}
}

func ensureYabaircHook() error {
	path := yabaircPath()
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(existing)
	if hasLegacyHook(content) {
		content = stripHookLines(content, legacyOnly)
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			return err
		}
		fmt.Printf("hook: dropped lines from an earlier release in %s\n", path)
	}

	if strings.Contains(content, hookLine) {
		fmt.Printf("hook: %s already wired\n", path)
		return nil
	}

	if len(strings.TrimSpace(content)) > 0 {
		backup := path + ".bak"
		if _, err := os.Stat(backup); os.IsNotExist(err) {
			if err := os.WriteFile(backup, []byte(content), 0o755); err != nil {
				return fmt.Errorf("backup %s: %w", path, err)
			}
			fmt.Printf("hook: backed up %s to %s\n", path, backup)
		}
	}

	block := "\n" + hookMarker + "\n" + hookLine + "\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(block); err != nil {
		return err
	}
	fmt.Printf("hook: appended the reconnect line to %s\n", path)
	return nil
}

const allLines, legacyOnly = true, false

func hasLegacyHook(content string) bool {
	if strings.Contains(content, legacyHookMarker) {
		return true
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), legacyHookLineTail) {
			return true
		}
	}
	return false
}

// stripHookLines removes the lines this tool added. legacyOnly keeps the current
// two lines in place, so installing over an older release only drops the old ones.
func stripHookLines(content string, includeCurrent bool) string {
	drop := map[string]bool{legacyHookMarker: true}
	if includeCurrent {
		drop[hookMarker] = true
		drop[hookLine] = true
	}
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if drop[line] || strings.HasSuffix(strings.TrimSpace(line), legacyHookLineTail) {
			continue
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, "\n")
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return out
}

func removeYabaircHook() {
	path := yabaircPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	stripped := stripHookLines(string(data), allLines)
	if stripped == string(data) {
		return
	}
	os.WriteFile(path, []byte(stripped), 0o755)
	fmt.Printf("hook: removed the reconnect lines from %s\n", path)
}

func teardown() {
	if err := removeSignals(); err != nil {
		fmt.Fprintf(os.Stderr, "signals: %v\n", err)
	} else {
		fmt.Println("signals: removed all yabai signals")
	}
	removeYabaircHook()
	if err := os.RemoveAll(cacheDir()); err != nil {
		fmt.Fprintf(os.Stderr, "cache: %v\n", err)
	} else {
		fmt.Printf("cache: removed %s\n", cacheDir())
	}
}

//-------------- doctor --------------

type doctorCheck struct {
	name string
	ok   bool
	note string
	fix  string
}

func doctor() {
	var checks []doctorCheck
	add := func(name string, ok bool, note, fix string) {
		checks = append(checks, doctorCheck{name: name, ok: ok, note: note, fix: fix})
	}

	for _, tool := range []string{"brew", "yabai", "sketchybar"} {
		path, err := exec.LookPath(tool)
		add("binary: "+tool, err == nil, path, "brew install "+tool)
	}
	if path, err := exec.LookPath("go"); err == nil {
		add("binary: go (build only)", true, path, "")
	} else {
		add("binary: go (build only)", false, "not found", "brew install go")
	}

	spaces, spacesErr := querySpaces()
	add("yabai scripting addition", spacesErr == nil,
		describeErr(spacesErr, fmt.Sprintf("%d spaces visible", len(spaces))),
		"disable SIP partially and run `sudo yabai --load-sa` (see README)")

	_, displaysErr := queryDisplays()
	add("yabai display query", displaysErr == nil, describeErr(displaysErr, "ok"), "same as above")

	add("yabai running", processRunning("yabai"), "", "brew services start yabai")
	add("sketchybar running", processRunning("sketchybar"), "", "brew services start sketchybar")

	fontPath := findNerdFont()
	add("font: Hack Nerd Font", fontPath != "", fontPath,
		"brew install --cask font-hack-nerd-font")

	cfg := defaultConfig()
	raw, readErr := os.ReadFile(configPath())
	jsonErr := error(nil)
	if readErr != nil {
		jsonErr = readErr
	} else if err := json.Unmarshal(raw, &cfg); err != nil {
		jsonErr = err
	}
	add("config.json", jsonErr == nil, describeErr(jsonErr, fmt.Sprintf("%d apps configured, %d with icons", len(cfg.Apps), countIcons(cfg.Apps))),
		"fix the JSON in "+configPath())

	hookPresent := false
	if data, err := os.ReadFile(yabaircPath()); err == nil {
		hookPresent = strings.Contains(string(data), hookLine)
	}
	add("~/.yabairc reconnect hook", hookPresent, "", "make install (or add the two lines by hand: see README)")

	if hookFile, err := os.Stat(filepath.Join(installDir(), "yabai-hook.sh")); err == nil {
		add("yabai-hook.sh", true, hookFile.ModTime().Format(time.RFC3339), "")
	} else {
		add("yabai-hook.sh", false, "missing", "make install")
	}

	registered, signalsErr := registeredSignals()
	want := len(signalsFor(""))
	got := 0
	for _, sig := range signalsFor("") {
		if registered[signalLabel(sig.event)] {
			got++
		}
	}
	add("yabai signals", signalsErr == nil && got == want,
		describeErr(signalsErr, fmt.Sprintf("%d/%d registered", got, want)),
		"restart yabai (the hook re-registers them) or run `update_sketchybar setup`")

	info := loadDisplayInfo()
	bar, barErr := queryBar()
	barOK := barErr == nil && bar.Height == info.MenubarHeight && bar.Position == "top" && bar.Topmost == "on"
	add("bar geometry", barOK,
		describeErr(barErr, fmt.Sprintf("position=%s height=%d (menubar %d) topmost=%s", bar.Position, bar.Height, info.MenubarHeight, bar.Topmost)),
		"sketchybar --reload")

	bar, barErr = queryBar()
	if barErr == nil {
		ours := 0
		for _, item := range bar.Items {
			if isOurs(item) {
				ours++
			}
		}
		add("bar items", ours > 0, fmt.Sprintf("%d items", ours), "sketchybar --reload")
	}

	fmt.Println("sketchybar-config doctor")
	fmt.Println()
	failed := 0
	for _, c := range checks {
		mark := "✓"
		if !c.ok {
			mark = "✗"
			failed++
		}
		line := fmt.Sprintf("%s %s", mark, c.name)
		if c.note != "" {
			line += ": " + c.note
		}
		fmt.Println(line)
		if !c.ok && c.fix != "" {
			fmt.Println("    fix: " + c.fix)
		}
	}
	fmt.Println()
	if failed == 0 {
		fmt.Println("everything checks out.")
	} else {
		fmt.Printf("%d issue(s) left. See README for the full setup, or run `make install` again.\n", failed)
	}
}

func describeErr(err error, ok string) string {
	if err != nil {
		return err.Error()
	}
	return ok
}

func countIcons(apps []AppConfig) int {
	n := 0
	for _, a := range apps {
		if a.Icon != "" {
			n++
		}
	}
	return n
}

func processRunning(name string) bool {
	return exec.Command("pgrep", "-x", name).Run() == nil
}

func findNerdFont() string {
	home, _ := os.UserHomeDir()
	dirs := []string{home + "/Library/Fonts", "/Library/Fonts", "/System/Library/Fonts"}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if strings.Contains(name, "Hack") && strings.Contains(name, "Nerd") {
				return filepath.Join(dir, name)
			}
		}
	}
	return ""
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
)

const (
	menubarHeight   = 30
	numSpaces       = 5
	windowsPerSpace = 4
)

const pillActive = "0x50ffffff"

var spaceBadgeColors = [numSpaces]string{
	"0xcc6aadff", // blue
	"0xcca78bfa", // violet
	"0xcc5cc9b0", // teal
	"0xccf0b429", // amber
	"0xccf472b6", // rose
}

//-------------- Config (loaded from ~/.config/sketchybar/config.json) --------------

type Config struct {
	MaxTitleWords int         `json:"maxTitleWords"`
	Apps          []AppConfig `json:"apps"`
}

type AppConfig struct {
	ID        string `json:"id"`
	Icon      string `json:"icon"`
	HideTitle bool   `json:"hideTitle,omitempty"`
}

func loadConfig() Config {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(home + "/.config/sketchybar/config.json")
	if err != nil {
		return defaultConfig()
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid config.json: %v\n", err)
		return defaultConfig()
	}
	if cfg.MaxTitleWords <= 0 {
		cfg.MaxTitleWords = 3
	}
	for i := range cfg.Apps {
		cfg.Apps[i].Icon = parseIcon(cfg.Apps[i].Icon)
	}
	return cfg
}

func defaultConfig() Config {
	return Config{
		MaxTitleWords: 3,
		Apps: []AppConfig{
			{ID: "Google Chrome", Icon: "\U000F02AF"},
			{ID: "Safari", Icon: "\U000F0584"},
			{ID: "Firefox", Icon: "\U000F0239"},
			{ID: "Visual Studio Code", Icon: "\U000F0A1E"},
			{ID: "Cursor", Icon: "\U000F0A1E"},
			{ID: "Ghostty", Icon: "\uF489"},
			{ID: "Alacritty", Icon: "\uF489"},
			{ID: "Terminal", Icon: "\uF489"},
			{ID: "Warp", Icon: "\uF489"},
			{ID: "Finder", Icon: "\U000F0036"},
			{ID: "WeChat", Icon: "\U000F0611", HideTitle: true},
			{ID: "Slack", Icon: "\U000F04B1"},
			{ID: "zoom.us", Icon: "\U000F0568"},
			{ID: "Spotify", Icon: "\U000F04C7"},
		},
	}
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

// findApp matches a window against the config by .app bundle name,
// with fallback to yabai's display name.
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

//-------------- Bundle name resolution (pid → .app name) --------------

func resolveBundleNames() map[int]string {
	out, err := exec.Command("ps", "-e", "-o", "pid=,comm=").Output()
	if err != nil {
		return nil
	}
	result := make(map[int]string)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		spaceIdx := strings.IndexByte(line, ' ')
		if spaceIdx < 0 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(line[:spaceIdx]))
		if err != nil {
			continue
		}
		path := strings.TrimSpace(line[spaceIdx:])
		for _, seg := range strings.Split(path, "/") {
			if strings.HasSuffix(seg, ".app") {
				result[pid] = strings.TrimSuffix(seg, ".app")
				break
			}
		}
	}
	return result
}

//-------------- Yabai / Sketchybar types --------------

type Space struct {
	ID        int   `json:"id"`
	Index     int   `json:"index"`
	Windows   []int `json:"windows"`
	IsVisible bool  `json:"is-visible"`
}

type Window struct {
	ID    int    `json:"id"`
	PID   int    `json:"pid"`
	App   string `json:"app"`
	Title string `json:"title"`
}

type Bar struct {
	Items []string `json:"items"`
}

//-------------- Data persistence via sketchybar item name --------------
//
// Stores window-to-slot mapping in a hidden bar item's name so that
// animations are stable across updates.
// Format: "data.{w0}:{w1}:.../{w0}:{w1}:.../..."

func toDataID(data [][]int) string {
	spaces := make([]string, numSpaces)
	for si := range numSpaces {
		items := make([]string, windowsPerSpace)
		for wi := range windowsPerSpace {
			v := 0
			if si < len(data) && wi < len(data[si]) {
				v = data[si][wi]
			}
			items[wi] = itoa(v)
		}
		spaces[si] = strings.Join(items, ":")
	}
	return "data." + strings.Join(spaces, "/")
}

func fromDataID(id string) [][]int {
	parts := strings.SplitN(id, ".", 2)
	if len(parts) != 2 || parts[0] != "data" {
		return emptyData()
	}
	spaceParts := strings.Split(parts[1], "/")
	result := make([][]int, len(spaceParts))
	for i, sp := range spaceParts {
		winParts := strings.Split(sp, ":")
		result[i] = make([]int, len(winParts))
		for j, wp := range winParts {
			result[i][j], _ = strconv.Atoi(wp)
		}
	}
	return result
}

func emptyData() [][]int {
	return fromDataID(toDataID(nil))
}

//-------------- Helpers --------------

func itoa(v int) string { return strconv.Itoa(v) }

/** cleanTitle extracts a readable short title from a window title.
Strips app name suffixes, removes symbols, keeps first N whole words. */
func cleanTitle(raw string, maxWords int) string {
	for _, sep := range []string{" — ", " - ", " | "} {
		if i := strings.Index(raw, sep); i > 0 {
			raw = raw[:i]
			break
		}
	}
	var buf []rune
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '.' || r == '-' || r == '_' {
			buf = append(buf, r)
		}
	}
	words := strings.Fields(string(buf))
	if len(words) > maxWords {
		words = words[:maxWords]
	}
	return strings.Join(words, " ")
}

func indexOf(slice []int, val int) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}

func hasItem(items []string, name string) bool {
	for _, item := range items {
		if item == name {
			return true
		}
	}
	return false
}

func runSketchybar(args []string) error {
	cmd := exec.Command("sketchybar", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func queryJSON(name string, args []string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), yabaiTimeout)
	defer cancel()
	data, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return json.Unmarshal(data, out)
}

func tryLock(path string) *os.File {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		f.Close()
		return nil
	}
	return f
}

//-------------- Initialize bar structure --------------
//
// Creates the fixed bar item structure. Items per space:
//   space.{i}.num  — space number (always visible, doubles as shortcut hint)
//   space.{i}.0…3  — window slots
//   space.{i}      — bracket grouping the above
//   space.{i}.gap  — spacing between spaces

func initialize() error {
	var args []string
	push := func(a ...string) { args = append(args, a...) }

	push("--bar",
		"color=0x00000000",
		"position=top",
		"height="+itoa(menubarHeight),
		"margin=0", "y_offset=0", "corner_radius=0",
		"border_width=0", "blur_radius=0",
		"padding_left=0", "padding_right=0",
		"display=main", "topmost=window", "sticky=on", "font_smoothing=on",
	)

	push("--default",
		"updates=when_shown", "drawing=on",
		"icon=", "icon.drawing=on",
		"icon.font=Hack Nerd Font:Bold:14.0",
		"icon.color=0xffffffff",
		"icon.padding_left=0", "icon.padding_right=0",
		"label=", "label.drawing=on",
		"label.font=Helvetica:Normal:13.0",
		"label.color=0xffffffff",
		"label.padding_left=0", "label.padding_right=0",
		"background.drawing=on", "background.corner_radius=3",
		"background.padding_left=0", "background.padding_right=0",
		"background.color=0x00ffffff",
		"background.height="+itoa(menubarHeight),
	)

	for si := range numSpaces {
		padID := fmt.Sprintf("space.%d.lpad", si)
		push("--add", "item", padID, "center")
		push("--set", padID,
			"drawing=on",
			"icon.drawing=off",
			"label= ",
			"label.font=Helvetica:Normal:1.0",
			"label.color=0x00ffffff",
			"label.padding_left=2", "label.padding_right=2",
			"background.drawing=off",
		)

		numID := fmt.Sprintf("space.%d.num", si)
		push("--add", "item", numID, "center")
		push("--set", numID,
			"drawing=on",
			"icon=", "icon.drawing=off",
			"label="+itoa(si+1),
			"label.font=Helvetica:Bold:11.0",
			"label.color=0xffffffff",
			"label.padding_left=6", "label.padding_right=6",
			"background.drawing=on",
			"background.color="+spaceBadgeColors[si],
			"background.corner_radius=4",
			"background.height=18",
		)

		bracketItems := []string{padID, numID}
		for wi := range windowsPerSpace {
			id := fmt.Sprintf("space.%d.%d", si, wi)
			bracketItems = append(bracketItems, id)
			push("--add", "item", id, "center")
			push("--set", id,
				"drawing=on",
				"label=", "label.color=0xffffffff",
				"background.height="+itoa(menubarHeight),
				"background.color=0x00ffffff",
			)
		}

		spaceID := fmt.Sprintf("space.%d", si)
		push(append([]string{"--add", "bracket", spaceID}, bracketItems...)...)
		push("--set", spaceID,
			"background.color=0x00ffffff",
			"background.corner_radius=8",
			"background.height=26",
		)

		gapID := fmt.Sprintf("space.%d.gap", si)
		push("--add", "item", gapID, "center")
		push("--set", gapID,
			"label=",
			"label.padding_left=3", "label.padding_right=3",
			"background.drawing=on", "background.color=0x00ffffff",
			"background.padding_left=0", "background.padding_right=0",
		)
	}

	dataID := toDataID(nil)
	push("--add", "item", dataID, "center")
	push("--set", dataID, "drawing=off")

	return runSketchybar(args)
}

//-------------- Update bar content --------------

func update(cfg *Config, spaces []Space, windows []Window, bar Bar, bundleNames map[int]string) error {
	windowsByID := make(map[int]Window, len(windows))
	for _, w := range windows {
		windowsByID[w.ID] = w
	}

	var args []string
	push := func(a ...string) { args = append(args, a...) }

	var dataIDs []string
	for _, item := range bar.Items {
		if strings.HasPrefix(item, "data.") {
			dataIDs = append(dataIDs, item)
		}
	}
	data := emptyData()
	if len(dataIDs) > 0 {
		data = fromDataID(dataIDs[0])
	}
	for _, d := range dataIDs {
		push("--remove", d)
	}

	push("--animate", "sin", "10")
	push("--bar", "color=0x00000000", "position=top", "y_offset=2")

	for si := range numSpaces {
		spaceID := fmt.Sprintf("space.%d", si)
		numID := fmt.Sprintf("space.%d.num", si)

		var space *Space
		var spaceWindows []Window
		if si < len(spaces) {
			space = &spaces[si]
			for _, wID := range space.Windows {
				if w, ok := windowsByID[wID]; ok && findApp(cfg, w.App, bundleNames[w.PID]) != nil {
					spaceWindows = append(spaceWindows, w)
				}
			}
		}

		spaceActive := space != nil && space.IsVisible

		//-------------- Space left pad + number badge --------------
		padID := fmt.Sprintf("space.%d.lpad", si)
		numLabel := ""
		if space != nil {
			numLabel = itoa(space.Index)
		}
		badgeColor := spaceBadgeColors[si]
		if !spaceActive {
			badgeColor = strings.Replace(badgeColor, "0xcc", "0x70", 1)
		}
		push("--set", padID,
			"label.padding_left=2", "label.padding_right=2",
		)
		push("--set", numID,
			"label="+numLabel,
			"background.color="+badgeColor,
		)

		//-------------- Stable slot assignment --------------
		prev := data[si]
		for len(prev) < windowsPerSpace {
			prev = append(prev, 0)
		}
		next := make([]int, windowsPerSpace)
		for _, win := range spaceWindows {
			if idx := indexOf(prev, win.ID); idx != -1 && idx < windowsPerSpace {
				next[idx] = win.ID
			}
		}
		for _, win := range spaceWindows {
			if indexOf(prev, win.ID) == -1 {
				if idx := indexOf(next, 0); idx != -1 {
					next[idx] = win.ID
				}
			}
		}

		//-------------- Window slots --------------
		for wi := range windowsPerSpace {
			itemID := fmt.Sprintf("space.%d.%d", si, wi)
			wID := next[wi]
			win, hasWin := windowsByID[wID]

			if hasWin && wID != 0 {
				app := findApp(cfg, win.App, bundleNames[win.PID])
				label := ""
				if !app.HideTitle {
					label = cleanTitle(win.Title, cfg.MaxTitleWords)
				}

				iconWidth := 0
				if app.Icon != "" {
					iconWidth = 22
				}

				push("--set", itemID,
					"icon="+app.Icon,
					"icon.width="+itoa(iconWidth),
					"icon.color=0xb0ffffff",
					"icon.padding_left=4", "icon.padding_right=2",
					"label="+label,
					"label.color=0xffffffff",
					"label.padding_left=1", "label.padding_right=4",
					"background.color=0x00ffffff",
					"background.corner_radius=3",
					"background.padding_left=0", "background.padding_right=0",
				)
			} else {
				push("--set", itemID,
					"icon=", "icon.width=0",
					"icon.padding_left=0", "icon.padding_right=0",
					"label=", "label.padding_left=0", "label.padding_right=0",
					"background.color=0x00ffffff",
					"background.padding_left=0", "background.padding_right=0",
				)
			}
		}

		data[si] = next

		//-------------- Bracket background (pill) --------------
		bracketBg := "0x00ffffff"
		if spaceActive {
			bracketBg = pillActive
		}
		push("--set", spaceID,
			"background.color="+bracketBg,
		)

		//-------------- Gap between spaces --------------
		gapID := fmt.Sprintf("space.%d.gap", si)
		push("--set", gapID,
			"label=",
			"label.padding_left=3", "label.padding_right=3",
		)
	}

	nextDataID := toDataID(data)
	push("--add", "item", nextDataID, "center")
	push("--set", nextDataID, "drawing=off")

	return runSketchybar(args)
}

//-------------- Yabai signal setup/teardown --------------

var yabaiEvents = []string{
	"application_visible",
	"application_hidden",
	"window_created",
	"window_destroyed",
	"window_minimized",
	"window_deminimized",
	"window_title_changed",
	"space_changed",
	"display_added",
	"display_removed",
}

const signalLabelPrefix = "update_sketchybar__"

func installDir() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/sketchybar"
}

const yabaiTimeout = 5 * time.Second

func yabaiCmd(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), yabaiTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yabai", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const yabaircMarker = "# sketchybar-config: auto-registered signals"

func setup() {
	binary := installDir() + "/update_sketchybar"
	registerSignals(binary)
	ensureYabairc(binary)
}

func registerSignals(binary string) {
	for _, event := range yabaiEvents {
		label := signalLabelPrefix + event
		yabaiCmd("-m", "signal", "--remove", label)
	}
	for _, event := range yabaiEvents {
		label := signalLabelPrefix + event
		if err := yabaiCmd("-m", "signal", "--add",
			"event="+event, "label="+label, "action="+binary,
		); err != nil {
			fmt.Fprintf(os.Stderr, "failed to register signal %s: %v\n", event, err)
		} else {
			fmt.Printf("registered yabai signal: %s\n", event)
		}
	}
}

func ensureYabairc(binary string) {
	home, _ := os.UserHomeDir()
	rcPath := home + "/.yabairc"
	setupLine := binary + " setup &"

	existing, _ := os.ReadFile(rcPath)
	content := string(existing)
	if strings.Contains(content, yabaircMarker) {
		fmt.Println("~/.yabairc already has setup hook")
		return
	}

	block := "\n" + yabaircMarker + "\n" + setupLine + "\n"
	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot write ~/.yabairc: %v\n", err)
		return
	}
	defer f.Close()
	f.WriteString(block)
	fmt.Println("added setup hook to ~/.yabairc")
}

func teardown() {
	for _, event := range yabaiEvents {
		yabaiCmd("-m", "signal", "--remove", signalLabelPrefix+event)
	}
	fmt.Println("removed yabai signals")

	home, _ := os.UserHomeDir()
	rcPath := home + "/.yabairc"
	data, err := os.ReadFile(rcPath)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	var kept []string
	skip := false
	for _, line := range lines {
		if line == yabaircMarker {
			skip = true
			continue
		}
		if skip {
			skip = false
			continue
		}
		kept = append(kept, line)
	}
	result := strings.Join(kept, "\n")
	os.WriteFile(rcPath, []byte(result), 0755)
	fmt.Println("removed setup hook from ~/.yabairc")
}

//-------------- Main --------------

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			setup()
			return
		case "teardown":
			teardown()
			return
		}
	}

	lockFile := tryLock(os.TempDir() + "/sketchybar-update.lock")
	if lockFile == nil {
		return
	}
	defer lockFile.Close()

	cfg := loadConfig()
	start := time.Now()

	var (
		spaces      []Space
		windows     []Window
		bar         Bar
		bundleNames map[int]string
		errs        [4]error
		wg          sync.WaitGroup
	)

	wg.Add(4)
	go func() {
		defer wg.Done()
		errs[0] = queryJSON("yabai", []string{"-m", "query", "--spaces"}, &spaces)
	}()
	go func() {
		defer wg.Done()
		errs[1] = queryJSON("yabai", []string{"-m", "query", "--windows"}, &windows)
	}()
	go func() {
		defer wg.Done()
		errs[2] = queryJSON("sketchybar", []string{"--query", "bar"}, &bar)
	}()
	go func() {
		defer wg.Done()
		bundleNames = resolveBundleNames()
	}()
	wg.Wait()

	for _, err := range errs[:3] {
		if err != nil {
			fmt.Fprintf(os.Stderr, "query error: %v\n", err)
			os.Exit(1)
		}
	}

	queryDone := time.Now()

	needsInit := len(bar.Items) == 0
	if !needsInit && !hasItem(bar.Items, "space.0.lpad") {
		fmt.Fprintln(os.Stderr, "bar structure outdated, restart sketchybar: brew services restart sketchybar")
		os.Exit(0)
	}

	if needsInit {
		if err := initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "initialize error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := update(&cfg, spaces, windows, bar, bundleNames); err != nil {
		fmt.Fprintf(os.Stderr, "update error: %v\n", err)
		os.Exit(1)
	}

	done := time.Now()
	fmt.Printf("updated sketchybar. total=%dms query=%dms update=%dms\n",
		done.Sub(start).Milliseconds(),
		queryDone.Sub(start).Milliseconds(),
		done.Sub(queryDone).Milliseconds(),
	)
}

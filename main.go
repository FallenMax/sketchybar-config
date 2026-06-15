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

const displayCacheFile = "/tmp/sketchybar-display-builtin"

/** detectAndCacheDisplay uses swift to call CoreGraphics and caches
the result in a temp file. Only called during setup or display change events. */
func detectAndCacheDisplay() bool {
	out, err := exec.Command("swift", "-e",
		`import CoreGraphics; print(CGDisplayIsBuiltin(CGMainDisplayID()) != 0 ? "1" : "0")`).Output()
	builtin := false
	if err == nil {
		builtin = strings.TrimSpace(string(out)) == "1"
	}
	val := "0"
	if builtin {
		val = "1"
	}
	os.WriteFile(displayCacheFile, []byte(val), 0644)
	fmt.Printf("detected display: builtin=%v\n", builtin)
	return builtin
}

func isMainDisplayBuiltin() bool {
	data, err := os.ReadFile(displayCacheFile)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

const (
	menubarHeight   = 30
	numSpaces       = 5
	windowsPerSpace = 5
)

//-------------- Config (loaded from ~/.config/sketchybar/config.json) --------------

type Config struct {
	MaxTitleWords int         `json:"maxTitleWords"`
	Apps          []AppConfig `json:"apps"`
}

type AppConfig struct {
	ID        string `json:"id"`
	Icon      string `json:"icon"`
	Color     string `json:"color,omitempty"`
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
		cfg.MaxTitleWords = 2
	}
	for i := range cfg.Apps {
		cfg.Apps[i].Icon = parseIcon(cfg.Apps[i].Icon)
		if cfg.Apps[i].Color == "" {
			cfg.Apps[i].Color = "0xffffffff"
		}
	}
	return cfg
}

func defaultConfig() Config {
	return Config{
		MaxTitleWords: 2,
		Apps: []AppConfig{
			{ID: "Google Chrome", Icon: "\U000F02AF", Color: "0xfff1bf47"},
			{ID: "Safari", Icon: "\U000F0584", Color: "0xff4ba0e8"},
			{ID: "Firefox", Icon: "\U000F0239", Color: "0xffff6611"},
			{ID: "Visual Studio Code", Icon: "\U000F0A1E", Color: "0xff4b9ae9"},
			{ID: "Cursor", Icon: "\U000F0A1E", Color: "0xff4b9ae9"},
			{ID: "Ghostty", Icon: "\uF489", Color: "0xffcc822e"},
			{ID: "Alacritty", Icon: "\uF489", Color: "0xffcc822e"},
			{ID: "Terminal", Icon: "\uF489", Color: "0xffffffff"},
			{ID: "Warp", Icon: "\uF489", Color: "0xff01c1a7"},
			{ID: "Finder", Icon: "\U000F0036", Color: "0xff1abffb"},
			{ID: "WeChat", Icon: "\U000F0611", Color: "0xff10d962", HideTitle: true},
			{ID: "Slack", Icon: "\U000F04B1", Color: "0xffe01e5a"},
			{ID: "zoom.us", Icon: "\U000F0568", Color: "0xff2d8cff"},
			{ID: "Spotify", Icon: "\U000F04C7", Color: "0xff65d56e"},
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
	Items    []string `json:"items"`
	Position string   `json:"position"`
}

//-------------- Slot data persistence via "data" item label --------------
//
// Stores window-to-slot mapping in a fixed hidden item's label.
// Format: "{w0}:{w1}:.../{w0}:{w1}:.../..."

func encodeSlots(data [][]int) string {
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
	return strings.Join(spaces, "/")
}

func decodeSlots(s string) [][]int {
	if s == "" {
		return emptySlots()
	}
	spaceParts := strings.Split(s, "/")
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

func emptySlots() [][]int {
	result := make([][]int, numSpaces)
	for i := range result {
		result[i] = make([]int, windowsPerSpace)
	}
	return result
}

//-------------- Helpers --------------

func itoa(v int) string { return strconv.Itoa(v) }

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

/** cleanTitle extracts a readable short title from a window title.
Strips app name suffixes, splits on non-letter boundaries, keeps first N words.
CJK characters: every 2 chars = 1 word. */
func cleanTitle(raw string, maxWords int) string {
	for _, sep := range []string{" — ", " - ", " | "} {
		if i := strings.Index(raw, sep); i > 0 {
			raw = raw[:i]
			break
		}
	}

	var words []string
	var cur []rune

	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = nil
		}
	}

	for _, r := range raw {
		if isCJK(r) {
			if len(cur) > 0 && !isCJK(cur[0]) {
				flush()
			}
			cur = append(cur, r)
			if len(cur) >= 2 {
				flush()
			}
		} else if unicode.IsLetter(r) {
			if len(cur) > 0 && isCJK(cur[0]) {
				flush()
			}
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()

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
//   space.{i}.0…4  — window slots (slot 0 doubles as space number when empty)
//   space.{i}      — bracket grouping the above
//   space.{i}.gap  — separator between spaces

func initialize(builtinDisplay bool) error {
	var args []string
	push := func(a ...string) { args = append(args, a...) }

	if builtinDisplay {
		push("--bar",
			"color=0x00000000",
			"position=bottom",
			"height="+itoa(menubarHeight),
			"margin=0", "y_offset=0", "corner_radius=0",
			"border_width=0", "blur_radius=0",
			"padding_left=0", "padding_right=0",
			"display=main", "topmost=on", "sticky=on", "font_smoothing=on",
		)
	} else {
		push("--bar",
			"color=0x00000000",
			"position=top",
			"height="+itoa(menubarHeight),
			"margin=0", "y_offset=2", "corner_radius=0",
			"border_width=0", "blur_radius=0",
			"padding_left=0", "padding_right=0",
			"display=main", "topmost=window", "sticky=on", "font_smoothing=on",
		)
	}

	push("--default",
		"updates=when_shown", "drawing=on",
		"icon=", "icon.drawing=on",
		"icon.font=Hack Nerd Font:Bold:16.0",
		"icon.color=0xffffffff",
		"icon.padding_left=0", "icon.padding_right=0",
		"label=", "label.drawing=on",
		"label.font=Helvetica:Normal:14.0",
		"label.color=0xccffffff",
		"label.padding_left=0", "label.padding_right=0",
		"label.y_offset=-1",
		"background.drawing=on", "background.corner_radius=2",
		"background.padding_left=0", "background.padding_right=0",
		"background.color=0x00ffffff",
		"background.height="+itoa(menubarHeight-4),
	)

	for si := range numSpaces {
		var bracketItems []string
		for wi := range windowsPerSpace {
			id := fmt.Sprintf("space.%d.%d", si, wi)
			bracketItems = append(bracketItems, id)
			push("--add", "item", id, "center")
			push("--set", id,
				"drawing=on",
				"label=", "label.color=0xccffffff",
				"background.height=18",
				"background.color=0x00ffffff",
			)
		}

		spaceID := fmt.Sprintf("space.%d", si)
		push(append([]string{"--add", "bracket", spaceID}, bracketItems...)...)
		push("--set", spaceID,
			"background.color=0x4d000000",
			"background.corner_radius=9999",
			"background.height=22",
			"background.border_width=0",
			"background.border_color=0x00000000",
		)

		gapID := fmt.Sprintf("space.%d.gap", si)
		push("--add", "item", gapID, "center")
		push("--set", gapID,
			"label=",
			"label.padding_left=4", "label.padding_right=4",
			"background.drawing=on", "background.color=0x00ffffff",
			"background.padding_left=0", "background.padding_right=0",
		)
	}

	push("--add", "item", "data", "center")
	push("--set", "data", "drawing=off")

	return runSketchybar(args)
}

//-------------- Update bar content --------------

func update(cfg *Config, spaces []Space, windows []Window, bundleNames map[int]string, slotsData string) error {
	windowsByID := make(map[int]Window, len(windows))
	for _, w := range windows {
		windowsByID[w.ID] = w
	}

	var args []string
	push := func(a ...string) { args = append(args, a...) }

	data := decodeSlots(slotsData)

	for si := range numSpaces {
		spaceID := fmt.Sprintf("space.%d", si)

		var space *Space
		var spaceWindows []Window
		if si < len(spaces) {
			space = &spaces[si]
			seen := make(map[int]bool)
			for _, wID := range space.Windows {
				if seen[wID] {
					continue
				}
				seen[wID] = true
				if w, ok := windowsByID[wID]; ok && w.Title != "" && findApp(cfg, w.App, bundleNames[w.PID]) != nil {
					spaceWindows = append(spaceWindows, w)
				}
			}
		}

		spaceActive := space != nil && space.IsVisible

		//-------------- Stable slot assignment (slots 1-4 for windows) --------------
		prev := data[si]
		for len(prev) < windowsPerSpace {
			prev = append(prev, 0)
		}
		next := make([]int, windowsPerSpace)
		for _, win := range spaceWindows {
			if idx := indexOf(prev, win.ID); idx > 0 && idx < windowsPerSpace {
				next[idx] = win.ID
			}
		}
		for _, win := range spaceWindows {
			if indexOf(next, win.ID) == -1 {
				for idx := 1; idx < windowsPerSpace; idx++ {
					if next[idx] == 0 {
						next[idx] = win.ID
						break
					}
				}
			}
		}

		//-------------- Render slots --------------
		iconColor := "0xff9ca3af" // gray-400
		if spaceActive {
			iconColor = "0xff60a5fa" // blue-400
		}

		for wi := range windowsPerSpace {
			itemID := fmt.Sprintf("space.%d.%d", si, wi)

			if wi == 0 {
				numLabel := ""
				if space != nil {
					numLabel = itoa(space.Index)
				}
				numColor := "0xff9ca3af"
				if spaceActive {
					numColor = "0xffffffff"
				}
				push("--set", itemID,
					"icon=", "icon.width=0",
					"icon.padding_left=0", "icon.padding_right=0",
					"label="+numLabel,
					"label.color="+numColor,
					"label.padding_left=8", "label.padding_right=4",
					"background.color=0x00ffffff",
					"background.padding_left=0", "background.padding_right=0",
				)
			} else {
				wID := next[wi]
				win, hasWin := windowsByID[wID]

				if hasWin && wID != 0 {
					app := findApp(cfg, win.App, bundleNames[win.PID])
					push("--set", itemID,
						"icon="+app.Icon,
						"icon.width=20",
						"icon.color="+iconColor,
						"icon.padding_left=2", "icon.padding_right=6",
						"label=", "label.padding_left=0", "label.padding_right=0",
						"background.color=0x00ffffff",
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
		}

		data[si] = next

		//-------------- Bracket: accent border on active, dim on inactive --------------
		if spaceActive {
			push("--set", spaceID,
				"background.color=0x99000000",
				"background.border_width=2",
				"background.border_color=0xff3b82f6",
			)
		} else {
			push("--set", spaceID,
				"background.color=0x4d000000",
				"background.border_width=0",
				"background.border_color=0x00000000",
			)
		}

		//-------------- Gap between pills --------------
		gapID := fmt.Sprintf("space.%d.gap", si)
		push("--set", gapID,
			"label=",
			"label.padding_left=3", "label.padding_right=3",
			"background.padding_left=0", "background.padding_right=0",
		)
	}

	push("--set", "data", "label="+encodeSlots(data))

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
	detectAndCacheDisplay()
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
		action := binary
		if event == "display_added" || event == "display_removed" {
			action = binary + " detect-display"
		}
		if err := yabaiCmd("-m", "signal", "--add",
			"event="+event, "label="+label, "action="+action,
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
		case "detect-display":
			detectAndCacheDisplay()
			// fall through to normal update
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
		builtinDisplay bool
		spaces         []Space
		windows        []Window
		bar            Bar
		slotsData      string
		bundleNames    map[int]string
		errs           [3]error
		wg             sync.WaitGroup
	)

	wg.Add(6)
	go func() {
		defer wg.Done()
		builtinDisplay = isMainDisplayBuiltin()
	}()
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
		var item struct {
			Label struct {
				Value string `json:"value"`
			} `json:"label"`
		}
		if queryJSON("sketchybar", []string{"--query", "data"}, &item) == nil {
			slotsData = item.Label.Value
		}
	}()
	go func() {
		defer wg.Done()
		bundleNames = resolveBundleNames()
	}()
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			fmt.Fprintf(os.Stderr, "query error: %v\n", err)
			os.Exit(1)
		}
	}

	queryDone := time.Now()

	needsInit := len(bar.Items) == 0

	expectedPos := "top"
	if builtinDisplay {
		expectedPos = "bottom"
	}
	if !needsInit && bar.Position != expectedPos {
		fmt.Printf("display mode changed (%s → %s), restarting sketchybar\n", bar.Position, expectedPos)
		exec.Command("brew", "services", "restart", "sketchybar").Run()
		return
	}

	if !needsInit && !hasItem(bar.Items, "space.0.0") {
		fmt.Fprintln(os.Stderr, "bar structure outdated, restart sketchybar: brew services restart sketchybar")
		os.Exit(0)
	}

	if needsInit {
		if err := initialize(builtinDisplay); err != nil {
			fmt.Fprintf(os.Stderr, "initialize error: %v\n", err)
			os.Exit(1)
		}
		slotsData = ""
		if builtinDisplay {
			yabaiCmd("-m", "config", "bottom_padding", itoa(menubarHeight+5))
		}
	}

	if err := update(&cfg, spaces, windows, bundleNames, slotsData); err != nil {
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

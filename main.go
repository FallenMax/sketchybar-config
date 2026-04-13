package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	macosMenubarHeight = 24
	numSpaces          = 5
	itemsInSpace       = 5
)

//-------------- Nerd Font icons (https://www.nerdfonts.com/cheat-sheet) --------------

const (
	iconChrome    = "\U000F02AF"
	iconCode      = "\U000F0A1E"
	iconFinder    = "\U000F0036"
	iconWeChat    = "\U000F0611"
	iconAlacritty = "\uF489"
	iconSpotify   = "\U000F04C7"
)

//-------------- Known apps --------------

type knownApp struct {
	name      string
	icon      string
	iconColor string
	getTitle  func(w Window) string
}

var knownApps = []knownApp{
	{
		name: "Google Chrome", icon: iconChrome, iconColor: "0xfff1bf47",
		getTitle: func(w Window) string {
			return truncate(strings.TrimSuffix(w.Title, " - Google Chrome"), 10)
		},
	},
	{
		name: "Code", icon: iconCode, iconColor: "0xff4b9ae9",
		getTitle: func(w Window) string {
			parts := strings.SplitN(w.Title, " — ", 2)
			project := parts[0]
			if len(parts) > 1 {
				project = parts[1]
			}
			return truncate(project, 10)
		},
	},
	{name: "访达", icon: iconFinder, iconColor: "0xff1abffb"},
	{
		name: "微信", icon: iconWeChat, iconColor: "0xff10d962",
		getTitle: func(Window) string { return "" },
	},
	{name: "Alacritty", icon: iconAlacritty, iconColor: "0xffcc822e"},
	{name: "Spotify", icon: iconSpotify, iconColor: "0xff65d56e"},
}

func findKnownApp(appName string) *knownApp {
	for i := range knownApps {
		if knownApps[i].name == appName {
			return &knownApps[i]
		}
	}
	return nil
}

//-------------- Yabai / Sketchybar types --------------

type Display struct {
	ID int `json:"id"`
}

type Space struct {
	ID        int   `json:"id"`
	Index     int   `json:"index"`
	Windows   []int `json:"windows"`
	IsVisible bool  `json:"is-visible"`
}

type Window struct {
	ID                int    `json:"id"`
	App               string `json:"app"`
	Title             string `json:"title"`
	HasFullscreenZoom bool   `json:"has-fullscreen-zoom"`
}

type Bar struct {
	Items []string `json:"items"`
}

//-------------- Data persistence via sketchybar item name --------------
//
// Hack: store window-to-slot mapping in a hidden bar item's name,
// so animations work correctly across updates.
// Format: "data.{w0}:{w1}:.../{w0}:{w1}:.../..." (/ separates spaces, : separates slots)

func toDataID(data [][]int) string {
	spaces := make([]string, numSpaces)
	for si := range numSpaces {
		items := make([]string, itemsInSpace)
		for wi := range itemsInSpace {
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

func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) > maxLen {
		return string(r[:maxLen])
	}
	return s
}

func indexOf(slice []int, val int) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}

func runSketchybar(args []string) error {
	cmd := exec.Command("sketchybar", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func queryJSON(name string, args []string, out any) error {
	data, err := exec.Command(name, args...).Output()
	if err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return json.Unmarshal(data, out)
}

//-------------- File lock (replaces external flock dependency) --------------

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

func initialize() error {
	var args []string
	push := func(a ...string) { args = append(args, a...) }

	push("--bar",
		"color=0xff131b20", "position=bottom",
		"height="+itoa(macosMenubarHeight),
		"margin=0", "y_offset=0", "corner_radius=5",
		"border_width=0", "blur_radius=50",
		"padding_left=0", "padding_right=0",
		"display=main", "topmost=on", "sticky=on", "font_smoothing=on",
	)

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
		"background.drawing=on", "background.corner_radius=2",
		"background.padding_left=0", "background.padding_right=0",
		"background.color=0x00ffffff",
		"background.height="+itoa(macosMenubarHeight-4),
	)

	for si := range numSpaces {
		spaceID := fmt.Sprintf("space.%d", si)
		items := make([]string, itemsInSpace)

		for wi := range itemsInSpace {
			id := fmt.Sprintf("space.%d.%d", si, wi)
			items[wi] = id
			push("--add", "item", id, "center")
			push("--set", id,
				"drawing=on",
				"label=", "label.color=0xccffffff",
				"background.height=18", "background.color=0x00ffffff",
			)
		}

		push(append([]string{"--add", "bracket", spaceID}, items...)...)
		push("--set", spaceID,
			"background.color=0x18ffffff",
			"background.corner_radius=4",
			"background.height="+itoa(macosMenubarHeight),
		)

		gapID := fmt.Sprintf("space.%d.gap", si)
		push("--add", "item", gapID, "center")
		push("--set", gapID,
			"label=", "label.padding_left=4", "label.padding_right=4",
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

func update(displays []Display, spaces []Space, windows []Window, bar Bar) error {
	isMacbook := len(displays) == 1

	windowsByID := make(map[int]Window, len(windows))
	for _, w := range windows {
		windowsByID[w.ID] = w
	}

	var args []string
	push := func(a ...string) { args = append(args, a...) }

	// Load previous window-to-slot mapping
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

	if isMacbook {
		push("--bar", "color=0xff131b20", "position=bottom")
	} else {
		push("--bar", "color=0x00272823", "position=top")
	}

	const labelColor = "0xccffffff"

	for si := range numSpaces {
		spaceID := fmt.Sprintf("space.%d", si)

		var space *Space
		var spaceWindows []Window
		if si < len(spaces) {
			space = &spaces[si]
			for _, wID := range space.Windows {
				if w, ok := windowsByID[wID]; ok && findKnownApp(w.App) != nil {
					spaceWindows = append(spaceWindows, w)
				}
			}
		}

		spaceEmpty := len(spaceWindows) == 0
		spaceActive := space != nil && space.IsVisible

		// Stable slot assignment: keep existing windows in their slots,
		// assign new windows to empty slots
		prev := data[si]
		next := make([]int, len(prev))
		for _, win := range spaceWindows {
			if idx := indexOf(prev, win.ID); idx != -1 {
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

		const rightPaddingFix = -3

		for wi := range itemsInSpace {
			itemID := fmt.Sprintf("space.%d.%d", si, wi)
			wID := next[wi]
			win, hasWin := windowsByID[wID]

			switch {
			case spaceEmpty && wi == 0 && space != nil:
				push("--set", itemID,
					"icon=", "icon.width=0",
					"icon.padding_left=0", "icon.padding_right=0",
					"label="+itoa(space.Index),
					"label.color="+labelColor,
					"label.padding_left=10",
					"label.padding_right="+itoa(10+rightPaddingFix),
					"background.color=0x00ffffff",
					"background.padding_left=0", "background.padding_right=0",
				)

			case hasWin && wID != 0:
				app := findKnownApp(win.App)
				label := truncate(win.Title, 10)
				if app.getTitle != nil {
					label = app.getTitle(win)
				}
				fs := win.HasFullscreenZoom

				iconWidth := 0
				if app.icon != "" {
					iconWidth = 26
				}
				iconColor := "0xffffffff"
				if app.iconColor != "" {
					iconColor = app.iconColor
				}
				lblColor := labelColor
				if spaceActive && fs {
					lblColor = "0xc0000000"
				} else if spaceActive {
					lblColor = "0xffffffff"
				}
				bgColor := "0x00ffffff"
				if spaceActive && fs {
					bgColor = "0xffffffff"
				}

				push("--set", itemID,
					"icon="+app.icon,
					"icon.width="+itoa(iconWidth),
					"icon.color="+iconColor,
					"icon.padding_left=8", "icon.padding_right=4",
					"label="+label,
					"label.color="+lblColor,
					"label.padding_left=4", "label.padding_right=8",
					"background.color="+bgColor,
					"background.padding_left=4",
					"background.padding_right="+itoa(4+rightPaddingFix),
				)

			default:
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

		bracketBg := "0x00ffffff"
		if spaceActive {
			bracketBg = "0x30ffffff"
		}
		push("--set", spaceID,
			"background.color="+bracketBg,
			"background.padding_left=0", "background.padding_right=0",
		)

		gapID := fmt.Sprintf("space.%d.gap", si)
		push("--set", gapID,
			"label=|",
			"label.padding_left=4", "label.padding_right=4",
			"label.color=0x30ffffff",
			"background.padding_left=4", "background.padding_right=4",
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

func setup() {
	binary := installDir() + "/update_sketchybar"

	// Remove stale signals first
	teardown()

	for _, event := range yabaiEvents {
		label := signalLabelPrefix + event
		cmd := exec.Command("yabai", "-m", "signal", "--add",
			"event="+event,
			"label="+label,
			"action="+binary,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to register signal %s: %v\n", event, err)
		} else {
			fmt.Printf("registered yabai signal: %s\n", event)
		}
	}
}

func teardown() {
	for _, event := range yabaiEvents {
		label := signalLabelPrefix + event
		exec.Command("yabai", "-m", "signal", "--remove", label).Run()
	}
	fmt.Println("removed yabai signals")
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

	home, _ := os.UserHomeDir()
	lockFile := tryLock(home + "/.config/sketchybar/update_sketchybar.lock")
	if lockFile == nil {
		return
	}
	defer lockFile.Close()

	start := time.Now()

	var (
		displays []Display
		spaces   []Space
		windows  []Window
		bar      Bar
		errs     [4]error
		wg       sync.WaitGroup
	)

	wg.Add(4)
	go func() {
		defer wg.Done()
		errs[0] = queryJSON("yabai", []string{"-m", "query", "--displays"}, &displays)
	}()
	go func() {
		defer wg.Done()
		errs[1] = queryJSON("yabai", []string{"-m", "query", "--spaces"}, &spaces)
	}()
	go func() {
		defer wg.Done()
		errs[2] = queryJSON("yabai", []string{"-m", "query", "--windows"}, &windows)
	}()
	go func() {
		defer wg.Done()
		errs[3] = queryJSON("sketchybar", []string{"--query", "bar"}, &bar)
	}()
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			fmt.Fprintf(os.Stderr, "query error: %v\n", err)
			os.Exit(1)
		}
	}

	queryDone := time.Now()

	if len(bar.Items) == 0 {
		if err := initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "initialize error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := update(displays, spaces, windows, bar); err != nil {
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

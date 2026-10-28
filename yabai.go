package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Space struct {
	ID        int   `json:"id"`
	Index     int   `json:"index"`
	Display   int   `json:"display"`
	Windows   []int `json:"windows"`
	IsVisible bool  `json:"is-visible"`
}

type Window struct {
	ID    int    `json:"id"`
	PID   int    `json:"pid"`
	App   string `json:"app"`
	Title string `json:"title"`
}

type Frame struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type Display struct {
	Index    int   `json:"index"`
	Frame    Frame `json:"frame"`
	HasFocus bool  `json:"has-focus"`
}

const yabaiTimeout = 5 * time.Second

func yabaiExec(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), yabaiTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yabai", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func yabaiQuery(args []string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), yabaiTimeout)
	defer cancel()
	data, err := exec.CommandContext(ctx, "yabai", args...).Output()
	if err != nil {
		return fmt.Errorf("yabai %s: %w", strings.Join(args, " "), err)
	}
	return json.Unmarshal(data, out)
}

func querySpaces() ([]Space, error) {
	var spaces []Space
	err := yabaiQuery([]string{"-m", "query", "--spaces"}, &spaces)
	return spaces, err
}

func queryWindows() ([]Window, error) {
	var windows []Window
	err := yabaiQuery([]string{"-m", "query", "--windows"}, &windows)
	return windows, err
}

func queryDisplays() ([]Display, error) {
	var displays []Display
	err := yabaiQuery([]string{"-m", "query", "--displays"}, &displays)
	return displays, err
}

// mainDisplayIndex returns the yabai display index of the display that carries
// the macOS menu bar, which is the display the "main" bar lives on. The main
// display sits at the origin of the global coordinate space; if that is not the
// case we fall back to the display with focus, then to the lowest index.
func mainDisplayIndex(displays []Display) int {
	if len(displays) == 0 {
		return 1
	}
	for _, d := range displays {
		if d.Frame.X == 0 && d.Frame.Y == 0 {
			return d.Index
		}
	}
	for _, d := range displays {
		if d.HasFocus {
			return d.Index
		}
	}
	return displays[0].Index
}

// spacesOnMainDisplay filters out spaces that belong to other displays, so the
// bar never mirrors state the user cannot see.
func spacesOnMainDisplay(spaces []Space, displays []Display) []Space {
	main := mainDisplayIndex(displays)
	kept := spaces[:0]
	for _, s := range spaces {
		if s.Display == main {
			kept = append(kept, s)
		}
	}
	return kept
}

// resolveBundleNames maps pid -> .app bundle name. Bundle names are language
// independent, unlike the display names yabai reports.
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

//-------------- yabai signals --------------
//
// yabai keeps signals in memory, so they are lost whenever yabai restarts. The
// install writes one line into ~/.yabairc which sources yabai-hook.sh, so every
// yabai start reconnects the bar. See install.go.

const signalPrefix = "sketchybar-config:"

// earlier releases used this prefix; drop those signals so the bar is not
// rendered twice while upgrading
const legacySignalPrefix = "update_sketchybar__"

type signal struct {
	event  string
	action string // "" means a plain render, otherwise a subcommand of this binary
}

func signalsFor(binary string) []signal {
	render := []string{
		"application_launched",
		"application_terminated",
		"application_visible",
		"application_hidden",
		"window_created",
		"window_destroyed",
		"window_moved",
		"window_minimized",
		"window_deminimized",
		"window_title_changed",
		"space_changed",
		"space_created",
		"space_destroyed",
		// leaving mission control is the only event yabai emits after a space
		// was added or removed from the UI
		"mission_control_exit",
	}
	redetect := []string{
		"display_added",
		"display_removed",
		"display_changed",
		"menu_bar_hidden_changed",
	}
	var out []signal
	for _, e := range render {
		out = append(out, signal{event: e, action: binary})
	}
	for _, e := range redetect {
		out = append(out, signal{event: e, action: binary + " detect-display"})
	}
	return out
}

func signalLabel(event string) string { return signalPrefix + event }

func registeredSignals() (map[string]bool, error) {
	var list []struct {
		Label string `json:"label"`
	}
	if err := yabaiQuery([]string{"-m", "signal", "--list"}, &list); err != nil {
		return nil, err
	}
	labels := make(map[string]bool, len(list))
	for _, s := range list {
		labels[s.Label] = true
	}
	return labels, nil
}

func registerSignals(binary string) error {
	registered, err := registeredSignals()
	if err != nil {
		return err
	}
	for label := range registered {
		if strings.HasPrefix(label, legacySignalPrefix) {
			if err := yabaiExec("-m", "signal", "--remove", label); err != nil {
				return err
			}
		}
	}
	for _, sig := range signalsFor(binary) {
		label := signalLabel(sig.event)
		if registered[label] {
			if err := yabaiExec("-m", "signal", "--remove", label); err != nil {
				return err
			}
		}
		if err := yabaiExec("-m", "signal", "--add",
			"event="+sig.event, "label="+label, "action="+sig.action); err != nil {
			return fmt.Errorf("register %s: %w", sig.event, err)
		}
	}
	return nil
}

func removeSignals() error {
	registered, err := registeredSignals()
	if err != nil {
		return err
	}
	var firstErr error
	for label := range registered {
		if !strings.HasPrefix(label, signalPrefix) && !strings.HasPrefix(label, legacySignalPrefix) {
			continue
		}
		if err := yabaiExec("-m", "signal", "--remove", label); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

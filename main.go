package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			setup()
			return
		case "teardown":
			teardown()
			return
		case "doctor":
			doctor()
			return
		case "summary":
			summary()
			return
		case "detect-display":
			info := detectAndCacheDisplay()
			fmt.Printf("display: builtin=%v menubar=%dpx\n", info.Builtin, info.MenubarHeight)
			// fall through: the display change also changes the layout
		}
	}
	update()
}

// update renders once. Concurrent invocations (yabai fires many events at once)
// coalesce: whoever loses the lock leaves a dirty marker behind, and the lock
// holder renders again afterwards instead of dropping the final state.
func update() {
	release, locked := acquireLock()
	if !locked {
		markDirty()
		return
	}
	defer release()

	for {
		// yabai fires events while the window server is still moving things
		// around; a short pause keeps the bar from rendering a half-applied
		// state (which shows up as a blink on space changes).
		time.Sleep(80 * time.Millisecond)
		if err := render(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		if !consumeDirty() {
			return
		}
	}
}

func acquireLock() (func(), bool) {
	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return func() {}, true
	}
	f, err := os.OpenFile(filepath.Join(cacheDir(), "update.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return func() {}, true
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, false
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, true
}

func dirtyPath() string { return filepath.Join(cacheDir(), "dirty") }

func markDirty() {
	os.MkdirAll(cacheDir(), 0o755)
	os.WriteFile(dirtyPath(), []byte("1"), 0o644)
}

func consumeDirty() bool {
	if _, err := os.Stat(dirtyPath()); err != nil {
		return false
	}
	os.Remove(dirtyPath())
	return true
}

func render() error {
	cfg := loadConfig()
	display := loadDisplayInfo()

	var (
		spaces   []Space
		windows  []Window
		displays []Display
		bar      BarInfo
		bundles  map[int]string
		errs     [4]error
		wg       sync.WaitGroup
	)

	// sketchybar can be busy handling a previous render and answer a query with
	// nothing, so every query gets a second chance.
	wg.Add(5)
	go func() {
		defer wg.Done()
		errs[0] = retry(func() (err error) { spaces, err = querySpaces(); return })
	}()
	go func() {
		defer wg.Done()
		errs[1] = retry(func() (err error) { windows, err = queryWindows(); return })
	}()
	go func() {
		defer wg.Done()
		errs[2] = retry(func() (err error) { displays, err = queryDisplays(); return })
	}()
	go func() {
		defer wg.Done()
		errs[3] = retry(func() (err error) { bar, err = queryBar(); return })
	}()
	go func() {
		defer wg.Done()
		bundles = resolveBundleNames()
	}()
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	spaces = spacesOnMainDisplay(spaces, displays)
	sort.Slice(spaces, func(i, j int) bool { return spaces[i].Index < spaces[j].Index })

	// yabai can answer with an empty space list while the window server is
	// rearranging things. Tearing the bar down for that would make it blink,
	// so keep the current state and wait for the next event.
	if len(spaces) == 0 {
		fmt.Fprintln(os.Stderr, "render: yabai reported no spaces on the main display, keeping the bar as is")
		return nil
	}

	windowsByID := make(map[int]Window, len(windows))
	for _, w := range windows {
		windowsByID[w.ID] = w
	}

	desired := desiredItems(spaces, cfg.MaxWindowsPerSpace)
	fingerprint := strings.Join(desired, " ")
	existing := filterOurs(bar.Items)

	state := loadState()
	// Rebuild only when the bar is empty (first run, sketchybar restarted) or
	// when the structure this tool owns has to change (space added/removed,
	// config edited). Never trust the live item list for that decision: it can
	// be stale while spaces are switching, and rebuilding then makes the bar
	// blink.
	rebuilt := len(existing) == 0 || state.Structure != fingerprint

	// Bar properties and item defaults are only applied when they are actually
	// missing. Sending them on every update forces sketchybar to relayout the
	// whole bar, which shows up as a flicker on every space or window change.
	if rebuilt || barOutdated(bar, display) {
		var setup []string
		setup = append(setup, barArgs(display.MenubarHeight, display.Builtin)...)
		if rebuilt {
			setup = append(setup, defaultArgs(display.MenubarHeight)...)
			setup = append(setup, structureArgs(bar.Items, spaces, cfg.MaxWindowsPerSpace)...)
		}
		if err := runSketchybar(setup); err != nil {
			return err
		}
	}

	active := activeSpace(spaces, state.Active)
	args, nextState := renderArgs(renderInput{
		cfg:     &cfg,
		display: display,
		spaces:  spaces,
		windows: windowsByID,
		bundles: bundles,
		state:   state,
		active:  active,
	})
	if err := runSketchybar(args); err != nil {
		return err
	}
	nextState.Structure = fingerprint
	saveState(nextState)

	visible := 0
	for _, sp := range spaces {
		if sp.IsVisible {
			visible++
		}
	}
	note := ""
	if rebuilt {
		note = " (structure rebuilt)"
	}
	fmt.Printf("rendered: %d spaces on the main display, %d windows, %d bar items%s\n",
		len(spaces), len(windowsByID), len(desired), note)
	return nil
}

// activeSpace prefers the space yabai reports as visible. During a space
// transition yabai briefly reports none, so the previously active space is kept
// instead of dropping the highlight and re-adding it a moment later.
func activeSpace(spaces []Space, previous int) int {
	var visible int
	for _, sp := range spaces {
		if sp.IsVisible {
			if visible == 0 || sp.Index < visible {
				visible = sp.ID
			}
		}
	}
	if visible != 0 {
		return visible
	}
	for _, sp := range spaces {
		if sp.ID == previous {
			return previous
		}
	}
	if len(spaces) > 0 {
		return spaces[0].ID
	}
	return 0
}

// barOutdated reports whether the bar itself has to be reconfigured.
func barOutdated(bar BarInfo, display DisplayInfo) bool {
	return bar.Position != "top" || bar.Height != display.MenubarHeight || bar.Topmost != "on"
}

func retry(fn func() error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(30 * time.Millisecond)
	}
	return err
}

func filterOurs(items []string) []string {
	var ours []string
	for _, item := range items {
		if isOurs(item) {
			ours = append(ours, item)
		}
	}
	return ours
}

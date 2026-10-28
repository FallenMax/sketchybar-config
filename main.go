package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

	windowsByID := make(map[int]Window, len(windows))
	for _, w := range windows {
		windowsByID[w.ID] = w
	}

	desired := desiredItems(spaces, cfg.MaxWindowsPerSpace)
	existing := filterOurs(bar.Items)
	if !sameItems(existing, desired) {
		args := append(barArgs(display.MenubarHeight, display.Builtin), defaultArgs(display.MenubarHeight)...)
		args = append(args, structureArgs(bar.Items, spaces, cfg.MaxWindowsPerSpace)...)
		if err := runSketchybar(args); err != nil {
			return err
		}
	}

	args, nextState := renderArgs(renderInput{
		cfg:     &cfg,
		display: display,
		spaces:  spaces,
		windows: windowsByID,
		bundles: bundles,
		state:   loadState(),
	})
	if err := runSketchybar(args); err != nil {
		return err
	}
	saveState(nextState)

	visible := 0
	for _, sp := range spaces {
		if sp.IsVisible {
			visible++
		}
	}
	fmt.Printf("rendered: %d spaces on the main display, %d windows, %d bar items\n",
		len(spaces), len(windowsByID), len(desired))
	return nil
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

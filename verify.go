package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// verify is the scripted evidence for the invariants this project depends on.
// It talks to the live bar only, so it needs the same environment the renderer
// needs (yabai + sketchybar running). Everything it asserts was a real
// regression at least once; see ARCHITECTURE.md for why each one matters.

type verifyCheck struct {
	name   string
	ok     bool
	detail string
}

const (
	// a pill covers at least this fraction of its pixels in every frame, so a
	// blank frame (the "pill disappears for a moment" regression) is caught
	frameCoverageMin = 0.6
	// the right inset is measured to the ink of the last glyph, so it can only
	// be bounded: never tighter than the left side, and at most one glyph worth
	// of slack wider
	insetSlackMax = 8.0
)

func verify() {
	var checks []verifyCheck
	add := func(name string, ok bool, format string, args ...any) {
		checks = append(checks, verifyCheck{name: name, ok: ok, detail: fmt.Sprintf(format, args...)})
	}

	tmp, err := os.MkdirTemp("", "sketchybar-config-verify")
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	// measure the bar this binary produces, not whatever the last install left
	// behind: a broken layout has to fail here, which only works if the render
	// above the checks actually ran
	if err := render(); err != nil {
		fmt.Fprintf(os.Stderr, "verify: render failed: %v\n", err)
		os.Exit(1)
	}

	cfg := loadConfig()
	display := loadDisplayInfo()
	spaces, err := querySpaces()
	displays, displaysErr := queryDisplays()
	bar, barErr := queryBar()
	if err != nil || displaysErr != nil || barErr != nil {
		fmt.Fprintf(os.Stderr, "verify: yabai/sketchybar not reachable: %v %v %v\n", err, displaysErr, barErr)
		os.Exit(1)
	}

	spaces = spacesOnMainDisplay(spaces, displays)
	if len(spaces) == 0 {
		fmt.Fprintln(os.Stderr, "verify: no spaces on the main display")
		os.Exit(1)
	}

	// 1. the bar sits where the placement rule says it should
	layout := layoutFor(displayWidth(displays))
	add("bar geometry", bar.Position == layout.position &&
		bar.YOffset == layout.yOffset &&
		bar.Height == display.MenubarHeight &&
		bar.Topmost == "on",
		"position=%s margin=%d y_offset=%d height=%d, expected position=%s y_offset=%d height=%d",
		bar.Position, bar.Margin, bar.YOffset, bar.Height,
		layout.position, layout.yOffset, display.MenubarHeight)

	// 2. the bar window only covers the pills: everything it covers is not
	// clickable, so a full-width overlay would swallow the whole menu bar
	if layout.position == "top" && len(spaces) > 0 {
		first, firstErr := rectOf(bracketID(spaces[0].ID))
		last, lastErr := rectOf(bracketID(spaces[len(spaces)-1].ID))
		contentErr := firstErr
		if contentErr == nil {
			contentErr = lastErr
		}
		hugs, detail := false, "content bounds unavailable"
		if contentErr == nil {
			needed := marginForContent(displayWidth(displays), first.x0, last.x1)
			hugs = bar.Margin <= minInt(first.x0, displayWidth(displays)-last.x1)
			hugs = hugs && bar.Margin >= needed-marginHysteresis
			detail = fmt.Sprintf("margin=%d, pills span %d..%d, tightest margin=%d", bar.Margin, first.x0, last.x1, needed)
		}
		add("bar window hugs the pills", contentErr == nil && hugs, "%s", detail)
	}

	// 3. the item structure matches the derived one (no stale slots left behind)
	desired := desiredItems(spaces, cfg.MaxWindowsPerSpace)
	live := filterOurs(bar.Items)
	structureOK := len(live) == len(desired)
	if structureOK {
		for i := range desired {
			if live[i] != desired[i] {
				structureOK = false
				break
			}
		}
	}
	add("bar structure", structureOK, "%d live items, %d expected", len(live), len(desired))

	// 4. only windows that are really on screen get an icon on the active space
	windows, windowsErr := queryWindows()
	bundles := resolveBundleNames()
	activeID := activeSpace(spaces, 0)
	expectedIcons, activeSpaceVisible := 0, false
	if windowsErr == nil {
		for _, sp := range spaces {
			if sp.ID != activeID {
				continue
			}
			activeSpaceVisible = sp.IsVisible
			seen := map[int]bool{}
			for _, wid := range sp.Windows {
				if seen[wid] {
					continue
				}
				seen[wid] = true
				w, ok := windowByID(windows, wid)
				if !ok || findApp(&cfg, w.App, bundles[w.PID]) == nil || !shownOnSpace(w, sp.IsVisible) {
					continue
				}
				expectedIcons++
			}
		}
	}
	drawnIcons, iconErr := countDrawnIcons(activeID, cfg.MaxWindowsPerSpace)
	if iconErr != nil {
		windowsErr = iconErr
	}
	wantIcons := expectedIcons
	if wantIcons > cfg.MaxWindowsPerSpace {
		wantIcons = cfg.MaxWindowsPerSpace
	}
	add("window icons", windowsErr == nil && drawnIcons == wantIcons,
		"%d icons drawn, %d windows on screen (space visible=%v)", drawnIcons, expectedIcons, activeSpaceVisible)

	// 5. the pills own the inset they claim on both sides
	// the bar animates in after being shown or reconfigured, so give it time to
	// settle before treating a frame as the baseline
	runSketchybar([]string{"--bar", "hidden=off"})
	time.Sleep(400 * time.Millisecond)

	shownPath := filepath.Join(tmp, "shown.png")
	hiddenPath := filepath.Join(tmp, "hidden.png")
	if err := captureScreen(shownPath); err != nil {
		add("screenshot", false, "%v", err)
		report(checks)
		return
	}
	shown, err := readPNG(shownPath)
	if err != nil {
		add("screenshot", false, "%v", err)
		report(checks)
		return
	}
	scale := float64(shown.Bounds().Dx()) / float64(displayWidth(displays))

	runSketchybar([]string{"--bar", "hidden=on"})
	time.Sleep(400 * time.Millisecond)
	hideErr := captureScreen(hiddenPath)
	runSketchybar([]string{"--bar", "hidden=off"})
	time.Sleep(400 * time.Millisecond)
	if hideErr != nil {
		add("screenshot", false, "%v", hideErr)
		report(checks)
		return
	}
	hidden, err := readPNG(hiddenPath)
	if err != nil {
		add("screenshot", false, "%v", err)
		report(checks)
		return
	}

	active := activeSpace(spaces, 0)
	pill, err := rectOf(bracketID(active))
	if err != nil {
		add("active pill", false, "%v", err)
		report(checks)
		return
	}
	mask, leftInset, rightInset, insetsOK := measurePill(shown, hidden, pill, display, scale)
	add("pill insets", insetsOK,
		"left %.1fpt right %.1fpt (allowed %.1f..%.1f)", leftInset, rightInset, leftInset-1, leftInset+insetSlackMax)

	// 6. a render never blanks the pills
	if len(mask) == 0 {
		add("no blank frame while rendering", false, "no pill pixels found")
	} else {
		frames, captureErr := captureDuringRender(tmp, 8)
		worst, worstIndex := 1.0, 0
		for i, frame := range frames {
			if frame.Bounds() != shown.Bounds() {
				captureErr = fmt.Errorf("frame size changed")
				break
			}
			if ratio := stillPainted(mask, hidden, frame); ratio < worst {
				worst, worstIndex = ratio, i
			}
		}
		if captureErr != nil {
			add("no blank frame while rendering", false, "%v", captureErr)
		} else {
			add("no blank frame while rendering", worst >= frameCoverageMin,
				"worst frame %d kept %.0f%% of the pill pixels (min %.0f%%)", worstIndex+1, worst*100, frameCoverageMin*100)
		}
	}

	// 7. invalid colors fall back instead of rendering nothing
	fallbackOK := normalizeColor("0xzz", defaultColor) == defaultColor && normalizeColor(multicolor, defaultColor) == multicolor
	add("invalid color falls back", fallbackOK, "%s / %s", normalizeColor("0xzz", defaultColor), multicolor)

	report(checks)
}

func report(checks []verifyCheck) {
	fmt.Println("sketchybar-config verify")
	fmt.Println()
	failed := 0
	for _, c := range checks {
		mark := "✓"
		if !c.ok {
			mark = "✗"
			failed++
		}
		fmt.Printf("%s %s: %s\n", mark, c.name, c.detail)
	}
	fmt.Println()
	if failed > 0 {
		fmt.Printf("%d check(s) failed\n", failed)
		os.Exit(1)
	}
	fmt.Println("all checks passed")
}

func windowByID(windows []Window, id int) (Window, bool) {
	for _, w := range windows {
		if w.ID == id {
			return w, true
		}
	}
	return Window{}, false
}

// countDrawnIcons counts the window slots that currently show an icon.
func countDrawnIcons(spaceID, maxWindows int) (int, error) {
	count := 0
	for slot := 1; slot <= maxWindows; slot++ {
		out, err := exec.Command("sketchybar", "--query", slotID(spaceID, slot)).Output()
		if err != nil {
			return 0, err
		}
		var item struct {
			Icon struct {
				Value string `json:"value"`
			} `json:"icon"`
		}
		if err := json.Unmarshal(out, &item); err != nil {
			return 0, err
		}
		if item.Icon.Value != "" {
			count++
		}
	}
	return count, nil
}

func captureScreen(path string) error {
	return exec.Command("screencapture", "-x", path).Run()
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// captureDuringRender samples the screen while a render runs. The blank-frame
// regression only lasted one or two frames, so this has to keep sampling.
func captureDuringRender(dir string, count int) ([]image.Image, error) {
	var frames []image.Image
	done := make(chan error, 1)
	go func() {
		time.Sleep(80 * time.Millisecond)
		done <- render()
	}()
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("frame-%d.png", i))
		if err := captureScreen(path); err != nil {
			return frames, err
		}
		img, err := readPNG(path)
		if err != nil {
			return frames, err
		}
		frames = append(frames, img)
	}
	return frames, <-done
}

type bounds struct {
	x0, x1, y0, y1 int // in pixels
}

func rectOf(name string) (bounds, error) {
	out, err := exec.Command("sketchybar", "--query", name).Output()
	if err != nil {
		return bounds{}, err
	}
	var item struct {
		Rects map[string]struct {
			Origin [2]float64 `json:"origin"`
			Size   [2]float64 `json:"size"`
		} `json:"bounding_rects"`
	}
	if err := jsonUnmarshal(out, &item); err != nil {
		return bounds{}, err
	}
	for _, r := range item.Rects {
		return bounds{
			x0: int(math.Round(r.Origin[0])),
			x1: int(math.Round(r.Origin[0] + r.Size[0])),
			y0: int(math.Round(r.Origin[1])),
			y1: int(math.Round(r.Origin[1] + r.Size[1])),
		}, nil
	}
	return bounds{}, fmt.Errorf("no bounding rect for %s", name)
}

// measurePill isolates the pixels this tool paints inside one pill. Bounds come
// from the layout (a bracket rect spans the whole bar height), not from the
// difference image: hiding the bar can also change how the menu bar itself is
// composited, which would corrupt the bounds.
func measurePill(shown, hidden image.Image, pill bounds, display DisplayInfo, scale float64) ([]image.Point, float64, float64, bool) {
	x0 := int(math.Round(float64(pill.x0) * scale))
	x1 := int(math.Round(float64(pill.x1) * scale))
	// skip the rounded border rows at the very top and bottom of the pill
	pad := pillInsetY + 3
	y0 := int(math.Round(float64(pill.y0+pad) * scale))
	y1 := int(math.Round(float64(pill.y0+display.MenubarHeight-pad) * scale))
	edge := int(math.Round(4 * scale))

	var mask []image.Point
	inkMin, inkMax := math.MaxInt32, 0
	for y := y0; y < y1; y++ {
		for x := x0 + edge; x < x1-edge; x++ {
			if !differs(shown, hidden, x, y) || !isBright(shown, x, y) {
				continue
			}
			mask = append(mask, image.Point{X: x, Y: y})
			if x < inkMin {
				inkMin = x
			}
			if x > inkMax {
				inkMax = x
			}
		}
	}
	if inkMin == math.MaxInt32 {
		return nil, 0, 0, false
	}
	left := (float64(inkMin) - float64(x0)) / scale
	right := (float64(x1) - float64(inkMax)) / scale
	return mask, left, right, right >= left-1 && right <= left+insetSlackMax
}

// stillPainted reports the fraction of pill pixels the frame still paints, i.e.
// whether the pills are still on screen. Comparing against the "bar hidden"
// frame instead of raw colors keeps it stable while sketchybar animates.
func stillPainted(mask []image.Point, hidden, frame image.Image) float64 {
	if len(mask) == 0 {
		return 0
	}
	painted := 0
	for _, p := range mask {
		if differs(frame, hidden, p.X, p.Y) {
			painted++
		}
	}
	return float64(painted) / float64(len(mask))
}

// isBright separates content from the translucent pill fill behind it.
func isBright(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return (r>>8)+(g>>8)+(b>>8) > 200
}

func differs(a, b image.Image, x, y int) bool {
	ar, ag, ab, _ := a.At(x, y).RGBA()
	br, bg, bb, _ := b.At(x, y).RGBA()
	delta := math.Abs(float64(ar>>8)-float64(br>>8)) +
		math.Abs(float64(ag>>8)-float64(bg>>8)) +
		math.Abs(float64(ab>>8)-float64(bb>>8))
	return delta > 30
}

// jsonUnmarshal keeps the import list of this file small: the renderer already
// depends on encoding/json, reuse it here.
func jsonUnmarshal(data []byte, out any) error { return json.Unmarshal(data, out) }

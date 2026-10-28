package main

import (
	"fmt"
	"strings"
)

// Everything the bar renders is derived from yabai state, so the code below is
// stateless apart from the slot assignment in ~/.cache/sketchybar-config.

const (
	itemPrefix    = "space."
	legacyDataIDs = "data" // state used to be hidden in this item's label
)

// Layout metrics, in points.
//
// SketchyBar ignores icon padding when it lays out an item, and a bracket's own
// background padding does not extend what it draws. So every horizontal space
// comes from either icon.width (the box the glyph is centered in) or from label
// padding on the items at the edges of the pill.
const (
	pillInsetX = 10 // border to the first/last content
	pillInsetY = 1  // free space between bar edge and pill
	pillBorder = 1
	iconWidth  = 20 // icon box; glyphs are centered so this is also the spacing
	numberPadX = 7  // space between the space number and the first icon
	gapBetween = 8  // visible gap between two pills
	gapItemPad = gapBetween / 2
	numberYOff = -1
	endPadIcon = 5  // trailing spacer after the last window icon
	endPadText = 12 // trailing spacer after the +N counter
)

func slotID(spaceID, slot int) string { return fmt.Sprintf("%s%d.%d", itemPrefix, spaceID, slot) }
func moreID(spaceID int) string       { return fmt.Sprintf("%s%d.more", itemPrefix, spaceID) }
func endID(spaceID int) string        { return fmt.Sprintf("%s%d.end", itemPrefix, spaceID) }
func bracketID(spaceID int) string    { return fmt.Sprintf("%s%d", itemPrefix, spaceID) }
func gapID(spaceID int) string        { return fmt.Sprintf("%s%d.gap", itemPrefix, spaceID) }

func isOurs(name string) bool { return strings.HasPrefix(name, itemPrefix) || name == legacyDataIDs }

// desiredItems lists every component of the bar in creation order. Slot 0 of a
// space holds the space number, slots 1..max hold window icons, ".more" shows
// the overflow count.
func desiredItems(spaces []Space, maxWindows int) []string {
	var names []string
	for _, sp := range spaces {
		for slot := 0; slot <= maxWindows; slot++ {
			names = append(names, slotID(sp.ID, slot))
		}
		names = append(names, moreID(sp.ID))
		names = append(names, endID(sp.ID))
		names = append(names, bracketID(sp.ID))
		names = append(names, gapID(sp.ID))
	}
	return names
}

func bracketMembers(spaceID, maxWindows int) []string {
	members := make([]string, 0, maxWindows+2)
	for slot := 0; slot <= maxWindows; slot++ {
		members = append(members, slotID(spaceID, slot))
	}
	return append(members, moreID(spaceID), endID(spaceID))
}

// structureArgs removes every component we own and recreates the bar from
// scratch. Only called when the expected structure changed (first start, space
// added or removed, maxWindowsPerSpace edited).
func structureArgs(existing []string, spaces []Space, maxWindows int) []string {
	var args []string
	for i := len(existing) - 1; i >= 0; i-- {
		if isOurs(existing[i]) {
			args = append(args, "--remove", existing[i])
		}
	}
	for _, sp := range spaces {
		for slot := 0; slot <= maxWindows; slot++ {
			args = append(args, "--add", "item", slotID(sp.ID, slot), "center")
		}
		args = append(args, "--add", "item", moreID(sp.ID), "center")
		args = append(args, "--add", "item", endID(sp.ID), "center")
		args = append(args, "--add", "bracket", bracketID(sp.ID))
		args = append(args, bracketMembers(sp.ID, maxWindows)...)
		args = append(args, "--add", "item", gapID(sp.ID), "center")
	}
	return args
}

func barArgs(h int, builtin bool) []string {
	args := []string{"--bar",
		"color=0x00000000",
		"position=top",
		"height=" + itoa(h),
		"margin=0", "y_offset=0",
		"corner_radius=0", "border_width=0", "blur_radius=0",
		"padding_left=0", "padding_right=0",
		"display=main", "topmost=on", "sticky=on", "font_smoothing=on", "shadow=off",
	}
	if builtin {
		// Keep items clear of the notch instead of moving the whole bar away
		// from the menu bar.
		args = append(args, "notch_display_height="+itoa(h), "notch_width=200")
	}
	return args
}

func defaultArgs(h int) []string {
	return []string{"--default",
		"updates=when_shown", "drawing=on",
		"icon=", "icon.drawing=on",
		"icon.font=Hack Nerd Font:Bold:16.0",
		"icon.color=0xffffffff",
		"icon.padding_left=0", "icon.padding_right=0",
		"label=", "label.drawing=on",
		"label.font=Helvetica:Normal:14.0",
		"label.color=0xccffffff",
		"label.padding_left=0", "label.padding_right=0",
		"label.y_offset=" + itoa(numberYOff),
		"background.drawing=on", "background.corner_radius=2",
		"background.padding_left=0", "background.padding_right=0",
		"background.color=0x00ffffff",
		"background.height=" + itoa(maxInt(h-8, 10)),
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// assignSlots keeps windows in their previous slot when possible and only moves
// new windows into free slots.
func assignSlots(prev []int, windows []Window, maxWindows int) []int {
	next := make([]int, maxWindows+1)
	for _, w := range windows {
		for slot := 1; slot <= maxWindows; slot++ {
			if slot < len(prev) && prev[slot] == w.ID {
				next[slot] = w.ID
				break
			}
		}
	}
	for _, w := range windows {
		if indexOf(next, w.ID) != -1 {
			continue
		}
		for slot := 1; slot <= maxWindows; slot++ {
			if next[slot] == 0 {
				next[slot] = w.ID
				break
			}
		}
	}
	return next
}

type renderInput struct {
	cfg     *Config
	display DisplayInfo
	spaces  []Space
	windows map[int]Window
	bundles map[int]string
	state   State
	active  int // space id highlighted as active
}

// renderArgs produces the item updates and the next slot state.
func renderArgs(in renderInput) ([]string, State) {
	cfg := in.cfg
	height := in.display.MenubarHeight

	// accent color for the pill border, the space number and the overflow count
	accentFor := func(active bool) string {
		if active {
			return cfg.Colors.Border
		}
		return withAlpha(cfg.Colors.Border, dimAlpha)
	}
	// icon color: one color for everything, or per-app colors in multicolor mode
	iconColorFor := func(app *AppConfig, active bool) string {
		color := cfg.Colors.Icon
		if color == multicolor {
			color = app.Color
			if color == "" {
				color = cfg.Colors.Border
			}
		}
		if active {
			return color
		}
		return withAlpha(color, dimAlpha)
	}

	// Only item properties are sent here. Bar properties and item defaults are
	// applied by the structure setup, because sending them on every update
	// forces sketchybar to relayout the whole bar and the pills visibly blink.
	args := []string{"--animate", "sin", "10"}

	nextState := State{Slots: map[string][]int{}, Active: in.active}
	for _, sp := range in.spaces {
		active := sp.ID == in.active
		accent := accentFor(active)

		var windows []Window
		seen := make(map[int]bool, len(sp.Windows))
		for _, wid := range sp.Windows {
			if seen[wid] {
				continue
			}
			seen[wid] = true
			w, ok := in.windows[wid]
			if !ok {
				continue
			}
			if findApp(cfg, w.App, in.bundles[w.PID]) == nil {
				continue
			}
			windows = append(windows, w)
		}

		prev := in.state.Slots[itoa(sp.ID)]
		next := assignSlots(prev, windows, cfg.MaxWindowsPerSpace)
		nextState.Slots[itoa(sp.ID)] = next

		placed := 0
		for slot := 1; slot <= cfg.MaxWindowsPerSpace; slot++ {
			if next[slot] != 0 {
				placed++
			}
		}
		overflow := len(windows) - placed

		// slot 0: space number
		numberRpad := pillInsetX
		if placed > 0 || overflow > 0 {
			numberRpad = numberPadX
		}
		args = append(args, "--set", slotID(sp.ID, 0),
			"icon=", "icon.width=0", "icon.padding_left=0", "icon.padding_right=0",
			"label="+itoa(sp.Index),
			"label.color="+accent,
			"label.padding_left="+itoa(pillInsetX), "label.padding_right="+itoa(numberRpad),
			"background.color=0x00ffffff", "background.padding_left=0", "background.padding_right=0",
		)

		// slots 1..max: window icons
		for slot := 1; slot <= cfg.MaxWindowsPerSpace; slot++ {
			itemID := slotID(sp.ID, slot)
			wID := next[slot]
			w, ok := in.windows[wID]
			if wID == 0 || !ok {
				args = append(args, "--set", itemID,
					"icon=", "icon.width=0", "icon.padding_left=0", "icon.padding_right=0",
					"label=", "label.padding_left=0", "label.padding_right=0",
					"background.color=0x00ffffff", "background.padding_left=0", "background.padding_right=0",
				)
				continue
			}
			app := findApp(cfg, w.App, in.bundles[w.PID])
			iconColor := iconColorFor(app, active)
			width := 0
			if app.Icon != "" {
				width = iconWidth
			}
			args = append(args, "--set", itemID,
				"icon="+app.Icon,
				"icon.width="+itoa(width),
				"icon.color="+iconColor,
				"icon.padding_left=0", "icon.padding_right=0",
				"label=", "label.padding_left=0", "label.padding_right=0",
				"background.color=0x00ffffff", "background.padding_left=0", "background.padding_right=0",
			)
		}

		// overflow indicator
		if overflow > 0 {
			args = append(args, "--set", moreID(sp.ID),
				"icon=", "icon.width=0", "icon.padding_left=0", "icon.padding_right=0",
				"label=+"+itoa(overflow),
				"label.color="+accent,
				"label.padding_left="+itoa(numberPadX), "label.padding_right=0",
				"background.color=0x00ffffff", "background.padding_left=0", "background.padding_right=0",
			)
		} else {
			args = append(args, "--set", moreID(sp.ID),
				"icon=", "icon.width=0", "icon.padding_left=0", "icon.padding_right=0",
				"label=", "label.padding_left=0", "label.padding_right=0",
				"background.color=0x00ffffff", "background.padding_left=0", "background.padding_right=0",
			)
		}

		// trailing spacer: label.width is the only reliable way to reserve
		// horizontal space, icon padding and bracket background padding are
		// ignored by the layout engine
		endWidth := endPadIcon
		if overflow > 0 {
			endWidth = endPadText
		}
		args = append(args, "--set", endID(sp.ID),
			"icon=", "icon.width=0", "icon.padding_left=0", "icon.padding_right=0",
			"label=", "label.width="+itoa(endWidth),
			"label.padding_left=0", "label.padding_right=0",
			"background.color=0x00ffffff", "background.padding_left=0", "background.padding_right=0",
		)

		// bracket: highlight the active space
		bracket := []string{"--set", bracketID(sp.ID),
			"background.height=" + itoa(maxInt(height-2*pillInsetY, 12)),
			"background.corner_radius=9999",
			"background.padding_left=0",
			"background.padding_right=0",
		}
		if active {
			bracket = append(bracket,
				"background.color=0x99000000",
				"background.border_width="+itoa(pillBorder),
				"background.border_color="+cfg.Colors.Border,
			)
		} else {
			bracket = append(bracket,
				"background.color=0x4d000000",
				"background.border_width=0",
				"background.border_color=0x00000000",
			)
		}
		args = append(args, bracket...)

		// gap between pills
		args = append(args, "--set", gapID(sp.ID),
			"icon=", "icon.width=0", "icon.padding_left=0", "icon.padding_right=0",
			"label=", "label.padding_left="+itoa(gapItemPad), "label.padding_right="+itoa(gapItemPad),
			"background.color=0x00ffffff", "background.padding_left=0", "background.padding_right=0",
		)
	}

	return args, nextState
}

func indexOf(slice []int, val int) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}

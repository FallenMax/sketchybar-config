package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State holds the only things the renderer remembers between runs: which window
// sits in which slot per space (so icons do not jump around) and which space was
// active last time (so a space transition, where yabai briefly reports no
// visible space, does not make the highlight blink).
type State struct {
	Slots  map[string][]int `json:"slots"`
	Active int              `json:"active"`
	// Structure is the item list this tool last built. It is compared with the
	// freshly derived structure so a stale answer from sketchybar (which happens
	// while the window server is rearranging spaces) never triggers a rebuild.
	Structure string `json:"structure"`
}

func statePath() string { return filepath.Join(cacheDir(), "slots.json") }

func loadState() State {
	state := State{Slots: map[string][]int{}}
	data, err := os.ReadFile(statePath())
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil {
		// older releases stored the slot map on its own
		legacy := map[string][]int{}
		if json.Unmarshal(data, &legacy) == nil {
			return State{Slots: legacy}
		}
		return State{Slots: map[string][]int{}}
	}
	if state.Slots == nil {
		state.Slots = map[string][]int{}
	}
	return state
}

func saveState(state State) {
	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	os.WriteFile(statePath(), data, 0o644)
}

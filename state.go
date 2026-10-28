package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SlotState remembers which window occupies which slot per space, so icons do
// not jump around while windows open and close. Keys are yabai space ids.
type SlotState map[string][]int

func statePath() string { return filepath.Join(cacheDir(), "slots.json") }

func loadState() SlotState {
	state := SlotState{}
	data, err := os.ReadFile(statePath())
	if err != nil {
		return state
	}
	json.Unmarshal(data, &state)
	return state
}

func saveState(state SlotState) {
	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	os.WriteFile(statePath(), data, 0o644)
}

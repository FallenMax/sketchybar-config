package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

type BarInfo struct {
	Items    []string `json:"items"`
	Position string   `json:"position"`
	Height   int      `json:"height"`
	Topmost  string   `json:"topmost"`
}

func runSketchybar(args []string) error {
	cmd := exec.Command("sketchybar", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// queryBar retries: sketchybar does not queue queries while it is applying a
// previous command and then answers with an empty response.
func queryBar() (BarInfo, error) {
	var (
		bar BarInfo
		err error
	)
	for attempt := 0; attempt < 5; attempt++ {
		var out []byte
		out, err = exec.Command("sketchybar", "--query", "bar").Output()
		if err == nil {
			if jsonErr := json.Unmarshal(out, &bar); jsonErr == nil {
				return bar, nil
			} else {
				err = fmt.Errorf("sketchybar --query bar: %w", jsonErr)
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	return bar, err
}

func itoa(v int) string { return strconv.Itoa(v) }

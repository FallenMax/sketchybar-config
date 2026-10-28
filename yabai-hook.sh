#!/usr/bin/env sh
# sketchybar-config: sourced from ~/.yabairc on every yabai start.
# yabai keeps its signals in memory only, so a yabai restart loses them; this
# hook re-registers them and redraws the bar. Owned by sketchybar-config.

sketchybar_config_hook() {
  BINARY="$HOME/.config/sketchybar/update_sketchybar"
  [ -x "$BINARY" ] || return 0

  n=0
  while ! yabai -m query --spaces >/dev/null 2>&1; do
    n=$((n + 1))
    [ "$n" -ge 10 ] && return 1
    sleep 1
  done

  "$BINARY" setup
  "$BINARY"
}

sketchybar_config_hook

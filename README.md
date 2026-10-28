# sketchybar-config

Workspace pills for [SketchyBar](https://github.com/FelixKratz/SketchyBar), driven by
[yabai](https://github.com/koekeishiya/yabai): one pill per space showing the space
number and one icon per window, with per-app colors and an overflow counter.

![screenshot](./screenshot.png)

## What it shows

- one pill per space on the main display, space number first
- one icon per window; icons are either all one color or colored per app
- the active space gets a colored border and a brighter space number
- windows that do not fit show as `+N`
- icons only, no window titles

## Requirements

- macOS with [Homebrew](https://brew.sh/)
- [sketchybar](https://github.com/FelixKratz/SketchyBar) — `brew tap FelixKratz/formulae && brew install sketchybar`
- [yabai](https://github.com/koekeishiya/yabai) with its **scripting addition loaded** —
  `brew install koekeishiya/formulae/yabai`, then follow the
  [installation wiki](https://github.com/koekeishiya/yabai/wiki/Installing-yabai-(latest-release)):
  partially disable SIP, add the sudoers entry and run `sudo yabai --load-sa`.
  `yabai -m query --spaces` must print JSON before this bar can work.
- `yabai` running and started at login — `brew services start yabai`
- "Displays have separate Spaces" enabled (System Settings → Desktop & Dock), a
  SketchyBar requirement
- [Hack Nerd Font](https://www.nerdfonts.com/) — `brew install --cask font-hack-nerd-font`
- Go, only to build — `brew install go`

## Install

```sh
git clone https://github.com/FallenMax/sketchybar-config.git
cd sketchybar-config
make install
make doctor      # verifies every dependency and integration point
```

`make install` builds the renderer and places it in `~/.config/sketchybar/`, then
starts (or restarts) the sketchybar service. When it finishes it prints exactly what
it changed on your system — see [It touches these places](#it-touches-these-places).

You should now see the pills in the middle of the macOS menu bar. If you do not, run
`make doctor`; every failed check comes with a fix.

## It touches these places

Nothing else on your system is modified. No yabai setting is written at runtime.

- `~/.config/sketchybar/update_sketchybar` — the renderer binary, replaced on install
- `~/.config/sketchybar/sketchybarrc` — entry point sketchybar runs, replaced on install
  (an existing foreign `sketchybarrc` is backed up to `sketchybarrc.bak.<timestamp>`)
- `~/.config/sketchybar/yabai-hook.sh` — reconnect logic, replaced on install
- `~/.config/sketchybar/config.json` — your app icon list, created once, never overwritten
- `~/.yabairc` — two lines appended once (a marker comment plus
  `. "$HOME/.config/sketchybar/yabai-hook.sh"`); the file is backed up before the first append
- yabai signals — registered in memory, re-registered every time yabai starts
- `~/.cache/sketchybar-config/` — display probe result and window slot state

`make uninstall` reverts all of the above except `config.json` and `*.bak.*` backups.

## How it connects to yabai

yabai keeps its signals in memory, so a config script cannot persist them. That is
why exactly one line is added to `~/.yabairc`: every time yabai starts, it sources
`yabai-hook.sh`, which waits for yabai, re-registers the signals and redraws the bar.

The resulting behaviour for each startup combination:

- sketchybar starts first, yabai already running — `sketchybarrc` connects immediately
- sketchybar starts first, yabai comes up later — `sketchybarrc` waits for yabai (up
  to 60s) and draws as soon as it answers
- yabai comes up after that window — yabai runs the hook from `~/.yabairc`, which
  reconnects and redraws
- yabai restarts (`brew services restart yabai`) — the hook runs again, signals are
  re-registered, the bar keeps working
- sketchybar restarts (`make reload`, `brew services restart sketchybar`) — the bar
  rebuilds itself from yabai state, window icons keep their slot
- display added/removed/changed, menu bar hidden or shown — re-probed and redrawn,
  without restarting anybody
- yabai is not running at all — the bar stays empty and the reason is logged, see
  [Troubleshooting](#troubleshooting)

The one thing this cannot see: `yabai -m space --create` does not emit any event, so a
space created that way shows up with the next event (switching space, opening a window)
or after `make reload`. Spaces created through Mission Control arrive through the
`mission_control_exit` event.

## Configuration

`~/.config/sketchybar/config.json` (copy of `config.default.json` on first install;
`config.default.json` in the repo is the list every new machine starts from):

```json
{
  "colors": {
    "border": "0xff60a5fa",
    "icon": "0xff60a5fa"
  },
  "maxWindowsPerSpace": 6,
  "apps": [
    { "id": "Google Chrome", "icon": "U+F02AF", "color": "0xfff1bf47" },
    { "id": "WeChat", "icon": "U+F0611", "color": "0xff10d962" },
    { "id": "*", "icon": "U+EB7F", "color": "0xff9ca3af" }
  ]
}
```

Colors are two independent knobs, both blue by default so the whole bar looks
consistent out of the box:

- `colors.border` — pill border, space number and overflow counter
- `colors.icon` — every window icon: either one color, or the string
  `"multicolor"` to use the per-app `color` from the app list
- spaces that are not active dim both colors, so the active space still stands out
- `maxWindowsPerSpace` — how many window icons fit in a pill; the rest become `+N`
- `apps[].id` — the `.app` bundle name (e.g. `"Finder"` from `Finder.app`), which is
  language independent, unlike the localized display name. Find it in `/Applications/`
  or with `ps -e -o comm=`
- `apps[].icon` — Nerd Font codepoint (`"U+F02AF"`) or a raw character. Check that the
  codepoint exists in your installed font, otherwise it renders blank
- `apps[].color` — icon color, only used when `colors.icon` is `"multicolor"`.
  Optional; apps without a color fall back to `colors.border`
- `"*"` — catch-all for apps that are not listed, so they still get an icon

Icons can be looked up at [nerdfonts.com/cheat-sheet](https://www.nerdfonts.com/cheat-sheet).

Apply changes with:

```sh
make reload
```

## Commands

```sh
make install     # build, install, connect to yabai, start the bar
make doctor      # check dependencies, integration points and the live bar
make reload      # re-run sketchybarrc (picks up config.json changes)
make uninstall   # disconnect from yabai, stop and remove the bar
```

## Troubleshooting

- the bar is empty
  - `make doctor` first. The usual reasons: yabai's scripting addition is not loaded
    (`yabai -m query --spaces` fails), yabai is not running, or the font is missing
- the bar disappeared after a yabai restart
  - the line in `~/.yabairc` is missing (check with `make doctor`). If you keep your
    yabai config somewhere else, add the same two lines there
- icons are blank squares
  - the codepoint is missing from your Nerd Font, or the font is not installed
- nothing at all after `make install`
  - look at the logs, they are the only place where errors surface:
    - renderer / sketchybarrc: `$(brew --prefix)/var/log/sketchybar/sketchybar.out.log`
    - signal actions (spawned by yabai): `/tmp/yabai_$(whoami).out.log` and `.err.log`
- you want the menu bar back for a moment
  - the bar overlays the macOS menu bar, so menu bar items behind it cannot be clicked.
    Hide it temporarily with `sketchybar --bar hidden=toggle`, or bind that to a hotkey
- a new space is missing
  - `yabai -m space --create` emits no event; switch to the space or run `make reload`

## Uninstall

```sh
make uninstall
```

Removes the binary, the hook script, the yabai signals, the two `~/.yabairc` lines and
the cache directory, then stops the service. `config.json`, `sketchybarrc` and any
`*.bak.*` backups stay behind so you can reinstall without losing your app list.

## How it works

- one Go binary, no runtime dependencies, talks to `yabai` and `sketchybar` only
- stateless: every render reads spaces, windows and displays from yabai and rebuilds
  the bar; the only persisted state is "which window sits in which slot"
  (`~/.cache/sketchybar-config/slots.json`), so icons do not jump around
- yabai signals trigger renders; concurrent events coalesce into one extra render
  instead of being dropped
- only spaces of the main display are drawn (one bar, one display)

My SketchyBar setup mentioned at https://github.com/FelixKratz/SketchyBar/discussions/47#discussioncomment-4808906

![screenshot](./screenshot.png)

Features

- Displays app titles and updates on change (with animation)
  - Instead of using spaces for fixed purposes, I allocate any new task to a free space. So it's nice to know how spaces are used and their purpose from a glance
- Co-exists with macOS native menubar
  - Native menubar is useful, also I'm using [MenubarX](https://menubarx.app/)

## Prerequisites

- macOS (Apple Silicon)
- [Homebrew](https://brew.sh/)
- [sketchybar](https://github.com/FelixKratz/SketchyBar) — `brew install FelixKratz/formulae/sketchybar`
- [yabai](https://github.com/koekeishiya/yabai) — `brew install koekeishiya/formulae/yabai`
- [Hack Nerd Font](https://www.nerdfonts.com/) — `brew install --cask font-hack-nerd-font`
- [Go](https://go.dev/) — `brew install go` (build only)

## Install

```sh
git clone https://github.com/user/sketchybar-config.git
cd sketchybar-config
make install
```

This will:
1. Build the binary
2. Install to `~/.config/sketchybar/`
3. Register yabai signals so the bar updates on window changes
4. Restart sketchybar

## Uninstall

```sh
make uninstall
```

## Customization

Edit `knownApps` in `main.go` to add/remove apps shown in the bar. Icons from [Nerd Fonts cheat sheet](https://www.nerdfonts.com/cheat-sheet).

Rebuild and reinstall after changes:

```sh
make install
```

My SketchyBar setup mentioned at https://github.com/FelixKratz/SketchyBar/discussions/47#discussioncomment-4808906

![screenshot](./screenshot.png)

Features

- Displays app titles and updates on change (with animation ✨)
  - Instead of using spaces for fixed purposes, I allocate any new task to a free space. So it's nice to know how spaces are used and their purpose from a glance
- Co-exists with macOS native menubar
  - Native menubar is useful, also I'm using [MenubarX](https://menubarx.app/)
- Written in TypeScript
  - Easier to maintain for complex logic (a.k.a. I can't write bash)

Usage:

- Install required dependencies:
  - node.js (run `npm install` or using yarn/pnpm)
  - yabai (query window info)
  - [flock](https://github.com/discoteq/flock) (to prevent multiple instances of the script)
- Modify code to your liking
  - Be sure to replace _"YOUR_USERNAME"_ to your own

Known issues:

- ~~After upgrading macos to Ventura, bar items jitter when update, no idea how to fix it yet~~
- Does not update when moving a window to another space using yabai, yabai seems not emit any event when this happens: [#1272](https://github.com/koekeishiya/yabai/issues/1272)

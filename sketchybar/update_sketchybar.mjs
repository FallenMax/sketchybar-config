#!/usr/bin/env zx

import { sketchybar_initialize } from './update_sketchybar_initialize.mjs'
import { sketchybar_update } from './update_sketchybar_update.mjs'
import { info } from './utils.mjs'

/**!
 * This script queries the window information of each space using yabai and
 * updates the sketchybar display
 *
 * Fixed structure in sketchybar terms:
 *
 * - **bracket** x 5 , representing 5 yabai spaces
 * - each bracket contains 5 **items**, representing either window title or spaceName(if no window)
 * - there's a special invisible **item** after each space, to provide gap between spaces
 *
 * The structure is built on the first run, then items are updated
 *     when there are changes to reduce flicker
 *
 * @see
 * - zx: https://github.com/google/zx
 * - sketchybar: https://felixkratz.github.io/SketchyBar/config/bar
 * - yabai: https://github.com/koekeishiya/yabai
 */
const main = async () => {
  const start = Date.now()

  // -------------- query windows/exist bar -----------------
  const beforeQuery = Date.now()
  // prettier-ignore
  const [displays, spaces, windows, bar] = (await Promise.all([
    $`yabai -m query --displays`,
    $`yabai -m query --spaces`,
    $`yabai -m query --windows`,
    $`sketchybar --query bar`
  ])).map((obj) => JSON.parse(String(obj)))
  info({ displays, spaces, windows, bar })

  const afterQuery = Date.now()

  // -------------- update config -----------------
  const beforeUpdate = Date.now()
  const hasInitialized = bar.items.length > 0
  if (!hasInitialized) {
    await sketchybar_initialize()
  }
  await sketchybar_update(displays, spaces, windows, bar)
  const afterUpdate = Date.now()

  console.info(`updated sketchybar. total=${Date.now() - start} query=${afterQuery - beforeQuery}ms, update=${afterUpdate - beforeUpdate}ms`)
}

$.verbose = false // do not print command
try {
  await main()
} catch (error) {
  console.error(error)
}

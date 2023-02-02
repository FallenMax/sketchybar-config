#!/usr/bin/env zx

import { sketchybar_initialize } from './update_sketchybar_initialize.mjs'
import { sketchybar_update } from './update_sketchybar_update.mjs'
import { info } from './utils.mjs'

/**!
 * this script query windows information on each spaces using yabai, and update
 * sketchybar display
 *
 * fixed structure:
 * - bracket1(space.0) to bracket5(space.4), each has 5 fixed items representing
 *   either window or spaceName, e.g.
 *   - item1(space.$s.0) to item5(space.$s.4)
 *   - item(space.$s.gap) (gap item after space, will not be included in
 *     bracket)
 *
 * we build the structure on the first run, then update the items when there are
 * changes
 *
 * @see
 * - zx: https://github.com/google/zx
 * - sketchybar: https://felixkratz.github.io/SketchyBar/config/bar
 * - yabai: https://github.com/koekeishiya/yabai
 */

$.verbose = false // do not print command

// -------------- constants -----------------

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

try {
  await main()
} catch (error) {
  console.error(error)
}

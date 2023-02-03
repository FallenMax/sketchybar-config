#!/usr/bin/env zx

import { $ } from 'zx'
import { Bar } from './sketchybar'
import { initialize } from './update_sketchybar_initialize'
import { update } from './update_sketchybar_update'
import { info } from './utils'
import { Display, Space, Window } from './yabai'

/**!
 * this script query windows information on each spaces using yabai, and update
 * sketchybar display
 *
 * bar items structures (they are fixed, so future updates are efficient and
 * animation works correctly):
 *
 * - 5 brackets: "space.0" to "space.4", each has 5 fixed items representing
 *   either window or spaceName:
 *   - windows: "space.$s.0" to "space.$s.4"
 *   - gap: "space.$s.gap"  (gap item after space, will not be included in
 *     bracket)
 * - a special item to store window information in its id
 *   - this is a hack to quickly store and retrieve previous window information, so that
 *     when we update windows, we can reuse the same `slot` for the same
 *     window to achieve correct animation
 *   - id format: `data.${space0}/${space1}/.../${space4}`
 *     - spaceX: `${yabaiWindowId}:${yabaiWindowId}:0:0:0` // 0 = no window
 *
 * we build the structure on the first run, then update the bar whenever something changes
 *
 * @see
 * - zx: https://github.com/google/zx
 * - sketchybar: https://felixkratz.github.io/SketchyBar/config/bar
 * - yabai: https://github.com/koekeishiya/yabai
 */

$.verbose = false // do not print command

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
  ])).map((obj) => JSON.parse(String(obj))) as [
    Display[],
    Space[],
    Window[],
    Bar
  ]
  info({ displays, spaces, windows, bar })

  const afterQuery = Date.now()

  // -------------- update config -----------------
  const beforeUpdate = Date.now()
  const hasInitialized = bar.items.length > 0
  if (!hasInitialized) {
    await initialize()
  }
  await update(displays, spaces, windows, bar)
  const afterUpdate = Date.now()

  console.info(`updated sketchybar. total=${Date.now() - start} query=${afterQuery - beforeQuery}ms, update=${afterUpdate - beforeUpdate}ms`)
}

try {
  await main()
} catch (error) {
  console.error(error)
}

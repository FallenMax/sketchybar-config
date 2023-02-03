import { $ } from 'zx'
import { ITEMS_IN_SPACE, MACOS_MENUBAR_HEIGHT, SPACES } from './consts'
import { info, toDataId, toParams } from './utils'

export async function initialize() {
  let args: string[] = []
  const flush = async () => {
    info(args)
    await $`sketchybar ${args}`
    args = []
  }
  const push = (argList: string[]) => {
    args.push(...argList)
  }

  // config bar
  {
    push([
      '--bar',
      ...toParams({
        color: '0xff131b20',
        position: 'bottom',
        height: MACOS_MENUBAR_HEIGHT,
        margin: 0,
        y_offset: 0,
        corner_radius: 5,
        border_width: 0,
        blur_radius: 50,
        padding_left: 0,
        padding_right: 0,
        display: 'main',
        topmost: 'on',
        sticky: 'on',
        font_smoothing: 'on',
      }),
    ])

    // config item defaults
    push([
      '--default',
      ...toParams({
        updates: 'when_shown',
        drawing: 'on',
        icon: {
          '': '',
          drawing: 'on',
          font: 'Hack Nerd Font:Bold:16.0',
          color: '0xffffffff',
          padding_left: 0,
          padding_right: 0,
        },
        label: {
          '': '',
          drawing: 'on',
          font: 'Helvetica:Normal:14.0',
          color: '0xccffffff',
          padding_left: 0,
          padding_right: 0,
        },
        background: {
          drawing: 'on',
          padding_left: 0,
          padding_right: 0,
          color: '0x00ffffff',
          corner_radius: 4,
          height: 20,
        },
      }),
    ])

    // spaces
    const labelColor = '0xccffffff'
    for (let spaceIndex = 0; spaceIndex < SPACES; spaceIndex++) {
      const spaceId = `space.${spaceIndex}`
      const items = []
      for (let itemIndex = 0; itemIndex < ITEMS_IN_SPACE; itemIndex++) {
        const itemId = `space.${spaceIndex}.${itemIndex}`
        items.push(itemId)
        push(['--add', 'item', itemId, 'center'])
        push([
          '--set',
          itemId,
          ...toParams({
            drawing: 'on',
            label: {
              '': '',
              color: labelColor,
            },
            background: {
              height: 18,
              color: '0x00ffffff',
            },
          }),
        ])
      }

      // group background
      {
        push(['--add', 'bracket', spaceId, ...items])
        push([
          '--set',
          spaceId,
          ...toParams({
            background: {
              color: '0x18ffffff',
              // color: '0xff00ff00',
              corner_radius: 9999,
              height: MACOS_MENUBAR_HEIGHT,
            },
          }),
        ])
      }

      // gap between spaces
      {
        const spaceGapId = `space.${spaceIndex}.gap`
        push(['--add', 'item', spaceGapId, 'center'])
        push([
          '--set',
          spaceGapId,
          ...toParams({
            label: {
              '': '',
              padding_left: 4,
              padding_right: 4,
            },
            background: {
              drawing: 'on',
              color: '0x00ffffff', // transparent
              // color: '0xffffffff',
              padding_left: 0,
              padding_right: 0,
            },
          }),
        ])
      }
    }

    // database
    {
      const databaseId = toDataId([])
      push(['--add', 'item', databaseId, 'center'])
      push([
        '--set',
        databaseId,
        ...toParams({
          drawing: 'off',
        }),
      ])
    }
  }

  await flush()
}

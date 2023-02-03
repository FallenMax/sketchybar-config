#!/usr/bin/env zx

import { $ } from 'zx'
import { ITEMS_IN_SPACE, MACOS_MENUBAR_HEIGHT, SPACES } from './consts'
import { Bar } from './sketchybar'
import { fromDataId, info, toDataId, toParams } from './utils'
import { Display, Space, Window } from './yabai'

// icons: https://www.nerdfonts.com/cheat-sheet
const KNOWN_APPS: KnownApp[] = [
  {
    app: 'Google Chrome',
    icon: '\udb80\udeaf',
    iconColor: '0xfff1bf47',
    getTitle(window) {
      const trimmed = window.title.replace(/ - Google Chrome$/, '')
      return trimmed.substr(0, 10)
    },
  },
  {
    app: 'Code',
    icon: '\udb82\ude1e',
    iconColor: '0xff4b9ae9',
    getTitle(window) {
      let [fileOrProject, project] = window.title.split(' — ')
      project = project || fileOrProject
      return project.substr(0, 10)
    },
  },
  {
    app: '访达',
    icon: '\udb80\udc36',
    iconColor: '0xff1abffb',
  },
  {
    app: '微信',
    icon: '\udb81\ude11',
    iconColor: '0xff10d962',
    getTitle() {
      return ''
    },
  },
  {
    app: 'Alacritty',
    icon: '\uf489',
    iconColor: '0xffcc822e',
  },
  {
    app: 'Spotify',
    icon: '\udb81\udcc7',
    iconColor: '0xff65d56e',
  },
]

type KnownApp = {
  app: string
  icon: string
  iconColor?: string
  getTitle?: (window: Window) => string
}
export async function update(displays: Display[], spaces: Space[], windows: Window[], bar: Bar) {
  const isMacbook = displays.length === 1
  const windowsById: Record<string, Window> = {}
  windows.forEach((w) => (windowsById[w.id] = w))

  // prepare params
  let args: string[] = []
  const flush = async () => {
    info(args)
    await $`sketchybar ${args}`
    args = []
  }
  const push = (argList: string[]) => {
    args.push(...argList)
  }

  {
    // load data
    const dataId = bar.items.find((item) => item.startsWith('data.'))
    const data = (dataId && fromDataId(dataId)) || fromDataId(toDataId([]))
    info(`old data id`, dataId)
    info(`old data`, data)
    if (dataId) {
      push(['--remove', dataId])
    }

    // enable animation
    push(['--animate', 'sin', '10'])

    // config bar
    push([
      '--bar',
      ...toParams({
        color: isMacbook ? '0xff131b20' : '0x00272823',
        position: isMacbook ? 'bottom' : 'top', // macbook has notch, so place it at bottom, with a dark bg
      }),
    ])

    // update spaces
    const labelColor = '0xccffffff'
    for (let spaceIndex = 0; spaceIndex < SPACES; spaceIndex++) {
      const spaceId = `space.${spaceIndex}`
      const space = spaces[spaceIndex]
      const windows = !space
        ? []
        : space.windows
            .map((wId) => windowsById[wId])
            .filter(Boolean)
            .filter((w) => KNOWN_APPS.find((app) => app.app === w.app))

      const spaceEmpty = windows.length === 0

      // here we'll update the same window by reusing the same slot,
      // then add new windows to available slots, and remove the rest
      const prevWindowIds = data[spaceIndex]

      // first remove everything
      const nextWindowIds = prevWindowIds.map((_) => 0)

      // try re-add windows that exists in last render
      windows.forEach((win) => {
        const oldIndex = prevWindowIds.indexOf(win.id)
        if (oldIndex !== -1) {
          nextWindowIds[oldIndex] = win.id
        }
      })

      // then add windows that are new
      windows.forEach((win) => {
        const oldIndex = prevWindowIds.indexOf(win.id)
        if (oldIndex === -1) {
          const emptyIndex = nextWindowIds.indexOf(0)
          if (emptyIndex !== -1) {
            nextWindowIds[emptyIndex] = win.id
          }
        }
      })

      // now that we have the nextWindowIds, we can update the data
      for (let itemIndex = 0; itemIndex < ITEMS_IN_SPACE; itemIndex++) {
        const itemId = `space.${spaceIndex}.${itemIndex}`
        const windowId = nextWindowIds[itemIndex]
        const window = windowId ? windowsById[windowId] : undefined

        // note: in order for updating to work correctly, when we update a item
        // (whatever role it has), we need to provide exactly the same set of
        // params, so that we can override the previous one
        interface ItemAttributes {
          icon: {
            '': string
            color?: string
            width: number
            padding_left: number
            padding_right: number
          }
          label: {
            '': string
            color?: string
            padding_left: number
            padding_right: number
          }
          background: {
            height: number
            padding_right: number
          }
        }

        // window name / space label / hidden
        if (spaceEmpty && itemIndex === 0) {
          const attrs: ItemAttributes = {
            icon: {
              '': '',
              width: 0,
              padding_left: 0,
              padding_right: 0,
            },
            label: {
              '': String(space.index),
              color: labelColor,
              padding_left: 10,
              padding_right: 6,
            },
            background: {
              height: 18,
              padding_right: 0,
            },
          }
          // space label, always takes the first slot
          // TODO maybe we should use a different item for space label
          push(['--set', itemId, ...toParams(attrs)])
        } else if (window) {
          // window name
          const matched = KNOWN_APPS.find((app) => app.app === window.app)!
          const label = matched.getTitle?.(window) ?? window.title.substr(0, 10)
          const attrs: ItemAttributes = {
            icon: {
              '': matched.icon,
              width: matched.icon ? 26 : 0,
              color: matched.iconColor || '0xffffffff',
              padding_left: 8,
              padding_right: 4,
            },
            label: {
              '': label,
              color: labelColor,
              padding_left: 0,
              padding_right: label ? 4 : 0,
            },
            background: {
              height: MACOS_MENUBAR_HEIGHT,
              padding_right: 4,
            },
          }
          push(['--set', itemId, ...toParams(attrs)])
        } else {
          // hidden
          const attrs: ItemAttributes = {
            icon: {
              '': '',
              width: 0,
              padding_left: 0,
              padding_right: 0,
            },
            label: {
              '': '',
              padding_left: 0,
              padding_right: 0,
            },
            background: {
              height: MACOS_MENUBAR_HEIGHT,
              padding_right: 0,
            },
          }
          push(['--set', itemId, ...toParams(attrs)])
        }
      }

      data[spaceIndex] = nextWindowIds

      // group background
      {
        const isActive = space?.['is-visible']
        push([
          '--set',
          spaceId,
          ...toParams({
            background: {
              color: isActive ? '0x30ffffff' : '0x18ffffff',
            },
          }),
        ])
      }

      // gap between spaces
      // {
      //   const spaceGapId = `space.${spaceIndex}.gap`
      //   push([
      //     '--set',
      //     spaceGapId,
      //     ...toParams({
      //       width: spaceExist ? 4 : 0,
      //     }),
      //   ])
      // }
    }

    // store data
    {
      const nextDataId = toDataId(data)
      info(`new data`, data)
      push(['--add', 'item', nextDataId, 'center'])
      push([
        '--set',
        nextDataId,
        ...toParams({
          drawing: 'off',
        }),
      ])
    }
  }

  await flush()
}

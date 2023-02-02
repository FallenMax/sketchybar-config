import { info, toParams } from './utils.mjs'

const MACOS_MENUBAR_HEIGHT = 24
const SPACES = 5
const ITEMS_IN_SPACE = 5

// icons: https://www.nerdfonts.com/cheat-sheet
const KNOWN_APPS = [
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

export async function sketchybar_update(displays, spaces, windows, bar) {
  const isMacbook = displays.length === 1
  const windowsById = {}
  windows.forEach((w) => (windowsById[w.id] = w))

  // prepare params
  let args = []
  const flush = async () => {
    info(args)
    await $`sketchybar ${args}`
    args = []
  }
  const push = (argList) => {
    args.push(...argList)
  }

  {
    // config bar
    // push(['--animate', 'linear', '10'])

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
      const spaceExist = space != null
      for (let itemIndex = 0; itemIndex < ITEMS_IN_SPACE; itemIndex++) {
        const itemId = `space.${spaceIndex}.${itemIndex}`

        // window name / space label / hidden
        if (spaceEmpty && itemIndex === 0) {
          // space label
          push([
            '--set',
            itemId,
            ...toParams({
              width: spaceExist ? MACOS_MENUBAR_HEIGHT : 0,
              icon: {
                '': '',
                width: 0,
                padding_left: 0,
                padding_right: 0,
              },
              label: {
                '': space.index,
                color: labelColor,
                padding_right: 0,
              },
              background: {
                height: 18,
                padding_right: 0,
              },
            }),
          ])
        } else {
          const window = windows[itemIndex]
          if (window) {
            // window name
            const matched = KNOWN_APPS.find((app) => app.app === window.app)
            push([
              '--set',
              itemId,
              ...toParams({
                width: 'dynamic',
                icon: {
                  '': matched.icon,
                  width: matched.icon ? 26 : 0,
                  color: matched.iconColor || '0xffffffff',
                  padding_left: 8,
                  padding_right: 4,
                },
                label: {
                  '': matched.getTitle?.(window) ?? window.title.substr(0, 10),
                  color: labelColor,
                  padding_right: 4,
                },
                background: {
                  height: MACOS_MENUBAR_HEIGHT,
                  padding_right: 4, // gap between app titles
                },
              }),
            ])
          } else {
            // hidden
            push([
              '--set',
              itemId,
              ...toParams({
                width: 0,
                icon: {
                  '': '',
                  width: 0,
                  padding_left: 0,
                  padding_right: 0,
                },
                label: {
                  '': '',
                  padding_right: 0,
                },
                background: {
                  padding_right: 0,
                },
              }),
            ])
          }
        }
      }

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
  }

  await flush()
}

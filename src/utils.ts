import { ITEMS_IN_SPACE, SPACES } from './consts'

// -------------- utils -----------------
// const info = console.info
export const info = (...args: any[]) => {}

export const toParams = (obj: Record<string, any>, prefix = ''): string[] => {
  let array: string[] = []
  Object.entries(obj).map(([k, v]) => {
    const nextPrefix = prefix && k ? prefix + '.' + k : k || prefix
    if (typeof v === 'object' && v !== null) {
      array.push(...toParams(v, nextPrefix))
    } else {
      array.push(`${nextPrefix}=${v}`)
    }
  })
  return array
}

export type WindowData = number[][]
export const toDataId = (windowGroups: WindowData) => {
  const id = `data.${new Array(SPACES)
    .fill(undefined)
    .map((_, spaceIndex) => {
      return new Array(ITEMS_IN_SPACE)
        .fill(undefined)
        .map((_, windowIndex) => {
          const window = windowGroups[spaceIndex]?.[windowIndex]
          return window || 0
        })
        .join(':')
    })
    .join('/')}`
  return id
}

export const fromDataId = (id: string): WindowData => {
  try {
    const [prefix, data] = id.split('.')
    if (prefix !== 'data') {
      throw new Error('Invalid database id')
    }
    const windowGroups = data.split('/').map((space) => {
      return space.split(':').map((window) => {
        return parseInt(window)
      })
    })
    return windowGroups
  } catch (error) {
    console.error('failed to parse database id', id, error)
    return []
  }
}

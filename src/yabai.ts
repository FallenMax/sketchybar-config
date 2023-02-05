export type Display = {
  id: string
}
export type Space = {
  id: string
  index: number
  windows: string[]
  ['is-visible']: boolean
}
export type Window = {
  id: number
  app: string
  title: string
  ['is-floating']: boolean
  ['is-sticky']: boolean
  ['is-topmost']: boolean
  ['has-fullscreen-zoom']: boolean
}

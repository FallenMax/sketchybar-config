// -------------- utils -----------------
// const info = console.info
export const info = () => {}

export const pick = (keys) => (obj) => {
  let picked = {}
  for (let key of keys) {
    picked[key] = obj[key]
  }
  return picked
}

export const toParams = (obj, prefix = '') => {
  let array = []
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

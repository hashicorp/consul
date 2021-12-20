export const isInternalLink = (link: string): boolean => {
  if (
    link.startsWith('/') ||
    link.startsWith('#') ||
    link.startsWith('https://consul.io') ||
    link.startsWith('https://www.consul.io')
  ) {
    return true
  }
  return false
}

export const chunk = (arr, chunkSize = 1, cache = []) => {
  const tmp = [...arr]
  if (chunkSize <= 0) return cache
  while (tmp.length) cache.push(tmp.splice(0, chunkSize))
  return cache
}

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

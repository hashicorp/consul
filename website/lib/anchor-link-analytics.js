// If there is a hash in the url, this script will check whether the hash matches
// the anchor link IDs for any element on the page and log it to our analytics.

export default function anchorLinkAnalytics() {
  if (
    typeof window === 'undefined' ||
    !window.requestIdleCallback ||
    !window.analytics
  )
    return

  window.requestIdleCallback(() => {
    const hash = window.location.hash
    if (hash.length < 1) return

    const targets = [].slice.call(
      document.querySelectorAll('.__target-lic, .__target-h')
    )
    const targetMatch = targets.find((t) => t.id === hash.replace(/^#/, ''))
    window.analytics.track('Anchor Link', { hash, hit: !!targetMatch })
  })
}

const withHashicorp = require('@hashicorp/platform-nextjs-plugin')
const redirects = require('./redirects.next')

module.exports = withHashicorp({
  dato: {
    // This token is safe to be in this public repository, it only has access to content that is publicly viewable on the website
    token: '88b4984480dad56295a8aadae6caad',
  },
  nextOptimizedImages: true,
  transpileModules: ['@hashicorp/flight-icons'],
})({
  svgo: { plugins: [{ removeViewBox: false }] },
  redirects: () => redirects,
  // Note: These are meant to be public, it's not a mistake that they are here
  env: {
    HASHI_ENV: process.env.HASHI_ENV || 'development',
    SEGMENT_WRITE_KEY: 'IyzLrqXkox5KJ8XL4fo8vTYNGfiKlTCm',
    BUGSNAG_CLIENT_KEY: '01625078d856ef022c88f0c78d2364f1',
    BUGSNAG_SERVER_KEY: 'be8ed0d0fc887d547284cce9e98e60e5',
    ENABLE_VERSIONED_DOCS: process.env.ENABLE_VERSIONED_DOCS || false,
  },
  images: {
    domains: ['www.datocms-assets.com'],
    disableStaticImages: true,
  },
})

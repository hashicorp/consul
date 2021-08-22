const withHashicorp = require('@hashicorp/platform-nextjs-plugin')
const redirects = require('./redirects.next')

module.exports = withHashicorp({
  transpileModules: ['@hashicorp/versioned-docs'],
})({
  svgo: { plugins: [{ removeViewBox: false }] },
  rewrites: () => [
    {
      source: '/api/:path*',
      destination: '/api-docs/:path*',
    },
  ],
  redirects: () => redirects,
  // Note: These are meant to be public, it's not a mistake that they are here
  env: {
    HASHI_ENV: process.env.HASHI_ENV || 'development',
    SEGMENT_WRITE_KEY: 'IyzLrqXkox5KJ8XL4fo8vTYNGfiKlTCm',
    BUGSNAG_CLIENT_KEY: '01625078d856ef022c88f0c78d2364f1',
    BUGSNAG_SERVER_KEY: 'be8ed0d0fc887d547284cce9e98e60e5',
  },
})

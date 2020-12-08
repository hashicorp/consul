const withHashicorp = require('@hashicorp/nextjs-scripts')
const path = require('path')

module.exports = withHashicorp({
  defaultLayout: true,
  transpileModules: ['is-absolute-url', '@hashicorp/react-.*'],
  mdx: { resolveIncludes: path.join(__dirname, 'pages/partials') },
})({
  svgo: { plugins: [{ removeViewBox: false }] },
  experimental: {
    modern: true,
    rewrites: () => [
      {
        source: '/api/:path*',
        destination: '/api-docs/:path*',
      },
    ],
  },
  // Note: These are meant to be public, it's not a mistake that they are here
  env: {
    HASHI_ENV: process.env.HASHI_ENV || 'development',
    SEGMENT_WRITE_KEY: 'IyzLrqXkox5KJ8XL4fo8vTYNGfiKlTCm',
    BUGSNAG_CLIENT_KEY: '01625078d856ef022c88f0c78d2364f1',
    BUGSNAG_SERVER_KEY: 'be8ed0d0fc887d547284cce9e98e60e5',
  },
})

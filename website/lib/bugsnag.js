import React from 'react'
import Bugsnag from '@bugsnag/js'
import BugsnagReact from '@bugsnag/plugin-react'

const apiKey =
  typeof window === 'undefined'
    ? 'be8ed0d0fc887d547284cce9e98e60e5' // server key
    : '01625078d856ef022c88f0c78d2364f1' // client key

if (!Bugsnag._client) {
  Bugsnag.start({
    apiKey,
    plugins: [new BugsnagReact(React)],
    otherOptions: { releaseStage: process.env.NODE_ENV || 'development' },
  })
}

export default Bugsnag

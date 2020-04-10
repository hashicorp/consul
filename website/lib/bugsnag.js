import React from 'react'
import bugsnag from '@bugsnag/js'
import bugsnagReact from '@bugsnag/plugin-react'

const apiKey =
  typeof window === 'undefined'
    ? 'be8ed0d0fc887d547284cce9e98e60e5'
    : '01625078d856ef022c88f0c78d2364f1'

const bugsnagClient = bugsnag({
  apiKey,
  releaseStage: process.env.NODE_ENV || 'development',
})

bugsnagClient.use(bugsnagReact, React)

export default bugsnagClient

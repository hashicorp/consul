import React from 'react'
import bugsnag from '@bugsnag/js'
import bugsnagReact from '@bugsnag/plugin-react'

const apiKey =
  typeof window === 'undefined'
    ? 'b6c57b27a37e531a5de94f065dd98bc0'
    : 'de0b822b269aa57b620efd8927e03744'

const bugsnagClient = bugsnag({
  apiKey,
  releaseStage: process.env.NODE_ENV || 'development',
})

bugsnagClient.use(bugsnagReact, React)

export default bugsnagClient

/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const read = require('fs').readFileSync;
const exec = require('child_process').execSync;

// See tests ../node-tests/config/utils.js
const repositoryYear = function (date = exec('git show -s --format=%ci HEAD')) {
  return date.toString().trim().split('-').shift();
};
const repositorySHA = function (sha = exec('git rev-parse --short HEAD')) {
  return sha.toString().trim();
};
const binaryVersion = function (repositoryRoot) {
  return function (versionFileContents = read(`${repositoryRoot}/version/VERSION`)) {
    // see /scripts/dist.sh:8
    return versionFileContents.toString();
  };
};
const env = function ($) {
  return function (flag, fallback) {
    // a fallback value MUST be set
    if (typeof fallback === 'undefined') {
      throw new Error(`Please provide a fallback value for $${flag}`);
    }
    // return the env var if set
    if (typeof $[flag] !== 'undefined') {
      if (typeof fallback === 'boolean') {
        // if we are expecting a boolean JSON parse strings to numbers/booleans
        return !!JSON.parse($[flag]);
      }
      return $[flag];
    }
    // If the fallback is a function call it and return the result.
    // Lazily calling the function means binaries used for fallback don't need
    // to be available if we are sure the environment variables will be set
    if (typeof fallback === 'function') {
      return fallback();
    }
    // just return the fallback value
    return fallback;
  };
};

module.exports = {
  repositoryYear: repositoryYear,
  repositorySHA: repositorySHA,
  binaryVersion: binaryVersion,
  env: env,
};

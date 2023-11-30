/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

// Technically this file configures babel transpilation support but we also
// use this file as a reference for our current browser support matrix and is
// therefore used by humans also. Therefore please feel free to be liberal
// with comments.

// We are moving to a rough ~2 years back support rather than a 2 versions
// back support. This strikes a balance between folks who need to get a job
// done in the Consul UI and keeping the codebase modern and being able to use
// modern Web Platform features. This is not set in stone but please consult
// with the rest of the team before bumping forwards (or backwards)
// We pin specific versions rather than use a relative value so we can choose
// to bump and it's clear what is supported.

///

// async/await support came before the below specified versions for Chrome,
// Firefox and Edge. Async/await is is the newest ES6 feature we are not
// transpiling. Safari's template literal support is a little problematic during
// v12 in that it has a GC bug for tagged template literals. We don't currently
// rely on this functionality so the bug wouldn't effect us, but in order to use
// browser versions as a measure for ES6 features we need to specify Safari 13
// for native, non-transpiled template literals. In reality template literals
// came in Safari 9.1. Safari's async/await support came in Safari 10.

module.exports = {
  browsers: ['Chrome 79', 'Firefox 72', 'Safari 13', 'Edge 79'],
};

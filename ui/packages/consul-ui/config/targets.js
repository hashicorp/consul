/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

// Technically this file configures babel transpilation support but we also
// use this file as a reference for our current browser support matrix and is
// therefore used by humans also. Therefore please feel free to be liberal
// with comments.

module.exports = {
  browsers: ['Chrome 79', 'Firefox 72', 'Safari 13', 'Edge 79'],
};

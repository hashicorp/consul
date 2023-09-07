/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
export default function (routes) {
  return function (name) {
    let wildcard = false;
    try {
      wildcard = get(routes, name)._options.path.indexOf('*') !== -1;
    } catch (e) {
      // passthrough
    }
    return wildcard;
  };
}

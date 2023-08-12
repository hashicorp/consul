/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (type) {
  return function (cb) {
    return function (params, hash = {}) {
      if (typeof params[0] !== type) {
        return params[0];
      }
      return cb(params[0], hash);
    };
  };
}

/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (obj) {
  if (typeof obj !== 'function') {
    return function () {
      return obj;
    };
  } else {
    return obj;
  }
}

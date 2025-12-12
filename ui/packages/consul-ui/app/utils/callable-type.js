/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
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

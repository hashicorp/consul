/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (prop) {
  return function (value) {
    if (typeof value === 'undefined' || value === null || value === '') {
      return {};
    } else {
      return {
        [prop]: value,
      };
    }
  };
}

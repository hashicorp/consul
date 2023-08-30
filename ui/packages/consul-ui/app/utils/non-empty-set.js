/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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

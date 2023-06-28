/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (e, value, target = {}) {
  if (typeof e.target !== 'undefined') {
    return e;
  }
  return {
    target: { ...target, ...{ name: e, value: value } },
  };
}

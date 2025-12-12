/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (e, value, target = {}) {
  if (typeof e.target !== 'undefined') {
    return e;
  }
  return {
    target: { ...target, ...{ name: e, value: value } },
  };
}

/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (doc = document) {
  return function (sel, context = doc) {
    return context.querySelectorAll(sel);
  };
}

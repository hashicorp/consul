/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (sel, el) {
  // basic DOM closest utility to cope with no support
  // TODO: instead of degrading gracefully
  // add a while polyfill for closest
  try {
    return el.closest(sel);
  } catch (e) {
    return null;
  }
}

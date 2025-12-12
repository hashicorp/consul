/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (el, name) {
  let sibling = el;
  while ((sibling = sibling.nextSibling)) {
    if (sibling.nodeType === 1) {
      if (sibling.nodeName.toLowerCase() === name) {
        return sibling;
      }
    }
  }
}

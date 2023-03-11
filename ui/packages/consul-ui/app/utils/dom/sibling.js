/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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

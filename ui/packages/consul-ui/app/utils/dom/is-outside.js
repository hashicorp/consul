/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (el, target, doc = document) {
  if (el) {
    // TODO: Potentially type check el and target
    // look to see what .contains does when it gets an unexpected type
    const isRemoved = !target || !doc.contains(target);
    const isInside = el === target || el.contains(target);
    return !isRemoved && !isInside;
  } else {
    return false;
  }
}

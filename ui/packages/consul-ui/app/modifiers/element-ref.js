/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

export default modifier(function elementRef(element, [callback]) {
  // Call the callback with the element when it's inserted
  if (callback && typeof callback === 'function') {
    callback(element);
  }

  // No cleanup needed for this simple case
  return () => {};
});

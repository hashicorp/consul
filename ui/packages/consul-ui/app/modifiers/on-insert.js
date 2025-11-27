/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

/**
 * A custom modifier to replace @ember/render-modifiers' did-insert.
 *
 * This modifier calls the provided callback function when the element is inserted into the DOM.
 * The element is passed as the first argument to the callback.
 *
 * Usage:
 *   {{on-insert this.handleInsert}}
 *   {{on-insert (fn this.handleInsert arg1 arg2)}}
 */
export default modifier((element, [callback, ...args]) => {
  if (typeof callback === 'function') {
    callback(element, ...args);
  }
});

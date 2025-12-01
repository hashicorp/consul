/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

/**
 * Custom modifier for outlet section element that:
 * 1. Captures the element reference on insertion
 * 2. Handles model updates
 *
 * Usage:
 * <section {{outlet-section this.captureElement this.updateModel @model}}></section>
 */
export default modifier((element, [onInsert, onUpdate, model]) => {
  // Call onInsert when element is first inserted (replaces did-insert)
  if (onInsert) {
    onInsert(element);
  }

  // Call onUpdate whenever model changes (replaces did-update)
  if (onUpdate) {
    onUpdate(model);
  }
});

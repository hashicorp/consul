/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';

export default class ClickOutsideModifier extends Modifier {
  @service dom;

  listeners = null;

  modify(element, positional, named) {
    const { enabled, excludeElements = [], onClickOutside } = named;

    // Remove existing listeners
    this.removeListener();

    // Only add listeners if enabled
    if (enabled) {
      this.addListener(element, excludeElements, onClickOutside);
    }
  }

  addListener(element, excludeElements, onClickOutside) {
    this.listeners = this.dom.listeners();
    this.listeners.add(this.dom.document(), 'click', (event) => {
      const target = event.target;

      // Check if click is outside the main element
      if (this.dom.isOutside(element, target)) {
        // Check if click is outside all excluded elements
        const isOutsideExcluded = excludeElements.every((excludeElement) => {
          return excludeElement ? this.dom.isOutside(excludeElement, target) : true;
        });

        if (isOutsideExcluded && onClickOutside) {
          onClickOutside(event);
        }
      }
    });
  }

  removeListener() {
    if (this.listeners) {
      this.listeners.remove();
      this.listeners = null;
    }
  }

  willDestroy() {
    super.willDestroy();
    this.removeListener();
  }
}

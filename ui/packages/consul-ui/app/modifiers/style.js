/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { assert } from '@ember/debug';

export default class StyleModifier extends Modifier {
  setStyles(newStyles = []) {
    const rulesToRemove = this._oldStyles || new Set();
    if (!Array.isArray(newStyles)) {
      newStyles = Object.entries(newStyles);
    }
    newStyles.forEach(([property, value]) => {
      assert(
        `Your given value for property '${property}' is ${value} (${typeof value}). Accepted types are string and undefined. Please change accordingly.`,
        typeof value === 'undefined' || typeof value === 'string'
      );

      // priority must be specified as separate argument
      // value must not contain "!important"
      let priority = '';
      if (value.length > 0 && value.includes('!important')) {
        priority = 'important';
        value = value.replace('!important', '');
      }

      // update CSSOM
      this.element.style.setProperty(property, value, priority);

      // should not remove rules that have been updated in this cycle
      rulesToRemove.delete(property);
    });

    // remove rules that were present in last cycle but aren't present in this one
    rulesToRemove.forEach((rule) => this.element.style.removeProperty(rule));

    // cache styles that in this rendering cycle for the next one
    this._oldStyles = new Set(newStyles.map((e) => e[0]));
  }

  didReceiveArguments() {
    if (typeof this.args.named.delay !== 'undefined') {
      setTimeout((_) => {
        if (typeof this !== this.args.positional[0]) {
          this.setStyles(this.args.positional[0]);
        }
      }, this.args.named.delay);
    } else {
      this.setStyles(this.args.positional[0]);
    }
  }
}

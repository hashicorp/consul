/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';

export default class CSSPropModifier extends Modifier {
  @service('-document') doc;

  modify(element, positional, named) {
    const returns = positional[1] || named?.returns;
    returns(this.doc.defaultView.getComputedStyle(element).getPropertyValue(positional[0]));
  }
}

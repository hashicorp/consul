/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';

export default class CSSPropModifier extends Modifier {
  @service('-document') doc;
  didReceiveArguments() {
    const params = this.args.positional;
    const options = this.args.named;
    const returns = params[1] || options.returns;
    returns(this.doc.defaultView.getComputedStyle(this.element).getPropertyValue(params[0]));
  }
}

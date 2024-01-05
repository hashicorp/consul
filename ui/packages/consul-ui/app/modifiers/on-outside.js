/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class OnOutsideModifier extends Modifier {
  @service('dom') dom;

  constructor() {
    super(...arguments);
    this.doc = this.dom.document();
  }
  async connect(params, options) {
    await new Promise((resolve) => setTimeout(resolve, 0));
    try {
      this.doc.addEventListener(params[0], this.listen);
    } catch (e) {
      // continue
    }
  }

  @action
  listen(e) {
    if (this.dom.isOutside(this.element, e.target)) {
      const dispatch = typeof this.params[1] === 'function' ? this.params[1] : (_) => {};
      dispatch.apply(this.element, [e]);
    }
  }

  disconnect() {
    this.doc.removeEventListener('click', this.listen);
  }

  didReceiveArguments() {
    this.params = this.args.positional;
    this.options = this.args.named;
  }

  didInstall() {
    this.connect(this.args.positional, this.args.named);
  }

  willRemove() {
    this.disconnect();
  }
}

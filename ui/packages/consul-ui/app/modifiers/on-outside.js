/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { registerDestructor } from '@ember/destroyable';

function cleanup(instance) {
  if (instance) {
    instance.doc?.removeEventListener('click', instance.listen);
  }
}

export default class OnOutsideModifier extends Modifier {
  @service('dom') dom;

  constructor(owner, args) {
    super(owner, args);
    this.doc = this.dom.document();

    registerDestructor(this, cleanup);
  }

  async modify(element, positional, named) {
    cleanup.call(this);

    this.params = positional;
    this.options = named;
    this.element = element;

    await new Promise((resolve) => setTimeout(resolve, 0));
    try {
      this.doc.addEventListener(positional[0], this.listen);
    } catch (e) {
      // continue
    }
  }

  @action
  listen(e) {
    if (this.element && this.dom.isOutside(this.element, e.target)) {
      const dispatch = typeof this.params[1] === 'function' ? this.params[1] : (_) => {};
      dispatch.apply(this.element, [e]);
    }
  }
}

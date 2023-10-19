/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';
import { registerDestructor } from '@ember/destroyable';

function cleanup(instance) {
  if (instance && instance?.named?.sticky) {
    instance.notify?.clearMessages();
  }
}
export default class NotificationModifier extends Modifier {
  @service('flashMessages') notify;

  modify(element, _, named) {
    this.named = named;
    element.setAttribute('role', 'alert');
    element.dataset['notification'] = null;

    const options = {
      timeout: 6000,
      extendedTimeout: 300,
      ...named,
    };
    options.dom = element.outerHTML;
    element.remove();
    this.notify.clearMessages();
    if (typeof options.after === 'function') {
      Promise.resolve()
        .then((_) => options.after())
        .catch((e) => {
          if (e.name !== 'TransitionAborted') {
            throw e;
          }
        })
        .then((_) => {
          this.notify.add(options);
        });
    } else {
      this.notify.add(options);
    }

    registerDestructor(this, cleanup);
  }
}

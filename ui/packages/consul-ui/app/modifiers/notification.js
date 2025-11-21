/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';
import { registerDestructor } from '@ember/destroyable';
import { scheduleOnce } from '@ember/runloop';

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
    const addMessage = () => {
      // clear + add after render so we don't mutate a value already consumed in current render
      this.notify.clearMessages();
      this.notify.add(options);
    };
    if (typeof options.after === 'function') {
      Promise.resolve()
        .then((_) => options.after())
        .catch((e) => {
          if (e.name !== 'TransitionAborted') {
            throw e;
          }
        })
        .then(() => scheduleOnce('afterRender', this, addMessage));
    } else {
      scheduleOnce('afterRender', this, addMessage);
    }

    registerDestructor(this, cleanup);
  }
}

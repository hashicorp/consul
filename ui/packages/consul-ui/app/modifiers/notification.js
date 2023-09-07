/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';

export default class NotificationModifier extends Modifier {
  @service('flashMessages') notify;

  didInstall() {
    this.element.setAttribute('role', 'alert');
    this.element.dataset['notification'] = null;
    const options = {
      timeout: 6000,
      extendedTimeout: 300,
      ...this.args.named,
    };
    options.dom = this.element.outerHTML;
    this.element.remove();
    this.notify.clearMessages();
    if (typeof options.after === 'function') {
      Promise.resolve()
        .then((_) => options.after())
        .catch((e) => {
          if (e.name !== 'TransitionAborted') {
            throw e;
          }
        })
        .then((res) => {
          this.notify.add(options);
        });
    } else {
      this.notify.add(options);
    }
  }
  willDestroy() {
    if (this.args.named.sticky) {
      this.notify.clearMessages();
    }
  }
}

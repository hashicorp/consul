/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { later, cancel as _cancel } from '@ember/runloop';
import { inject as service } from '@ember/service';

const DEFAULT_TIMEOUT = 10000;
const TESTING_TIMEOUT = 300;

export default class Watcher extends Component {
  @service env;

  @tracked _isPolling = false;
  @tracked cancel = null;

  get timeout() {
    if (this.isTesting) {
      return TESTING_TIMEOUT;
    } else {
      return this.args.timeout || DEFAULT_TIMEOUT;
    }
  }

  get isTesting() {
    return this.env.var('environment') === 'testing';
  }

  get isPolling() {
    const { isTesting, _isPolling: isPolling } = this;

    return !isTesting && isPolling;
  }

  @action start() {
    this._isPolling = true;

    this.watchTask();
  }

  @action stop() {
    this._isPolling = false;

    _cancel(this.cancel);
  }

  watchTask() {
    const cancel = later(
      this,
      () => {
        this.args.watch?.();

        if (this.isPolling) {
          this.watchTask();
        }
      },
      this.timeout
    );

    this.cancel = cancel;
  }
}

/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

import { fromPromise } from 'consul-ui/utils/dom/event-source';

// TODO: We could probably update this to be a template only component now
// rather than a JS only one.
export default class JWTSource extends Component {
  @service('repository/oidc-provider') repo;
  @service('dom') dom;

  constructor() {
    super(...arguments);
    if (this.source) {
      this.source.close();
    }
    this._listeners = this.dom.listeners();
    // TODO: Could this use once? Double check but I don't think it can
    this.source = fromPromise(this.repo.findCodeByURL(this.args.src));
    this._listeners.add(this.source, {
      message: (e) => this.onchange(e),
      error: (e) => this.onerror(e),
    });
  }

  onchange(e) {
    if (typeof this.args.onchange === 'function') {
      this.args.onchange(...arguments);
    }
  }

  onerror(e) {
    if (typeof this.args.onerror === 'function') {
      this.args.onerror(...arguments);
    }
  }

  willDestroy() {
    super.willDestroy(...arguments);
    if (this.source) {
      this.source.close();
    }
    this.repo.close();
    this._listeners.remove();
  }
}

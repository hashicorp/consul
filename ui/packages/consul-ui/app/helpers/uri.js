/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

const templateRe = /\${([A-Za-z.0-9_-]+)}/g;
let render;
export default class UriHelper extends Helper {
  @service('encoder') encoder;
  @service('data-source/service') data;

  constructor() {
    super(...arguments);
    if (typeof render !== 'function') {
      render = this.encoder.createRegExpEncoder(templateRe, encodeURIComponent);
    }
  }

  compute([template, vars]) {
    const str = render(template, vars);
    // `data.uri()` mints a new wrapper object per call, and DataSource/
    // DataLoader watch `@src` with `{{did-update-helper}}`, which fires on any
    // invalidation rather than on an actual change. Returning a stable
    // instance keeps an unchanged URI from reading as an update -- for
    // DataLoader that meant re-dispatching `LOAD` and tearing down everything
    // yielded to `:loaded`.
    if (this._str !== str) {
      this._str = str;
      this._uri = this.data.uri(str);
    }
    return this._uri;
  }
}

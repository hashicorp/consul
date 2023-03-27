/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Service, { inject as service } from '@ember/service';
import Clipboard from 'clipboard';

class ClipboardCallback extends Clipboard {
  constructor(trigger, options, cb) {
    super(trigger, options);
    this._cb = cb;
  }
  onClick(e) {
    this._cb(this.text(e.delegateTarget || e.currentTarget));
    // Clipboard uses/extends `tiny-emitter`
    // TODO: We should probably fill this out to match the obj passed from
    // os implementation
    this.emit('success', {});
  }
}

export default class LocalStorageService extends Service {
  @service('-document') doc;
  key = 'clipboard';

  execute(trigger, options) {
    return new ClipboardCallback(trigger, options, (val) => {
      this.doc.defaultView.localStorage.setItem(this.key, val);
    });
  }
}

import Service from '@ember/service';

import Clipboard from 'clipboard';

class ClipboardCallback extends Clipboard {
  constructor(trigger, cb) {
    super(trigger);
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

export default Service.extend({
  storage: window.localStorage,
  key: 'clipboard',
  execute: function(trigger) {
    return new ClipboardCallback(trigger, val => {
      this.storage.setItem(this.key, val);
    });
  },
});

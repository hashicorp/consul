import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  tagName: '',
  dom: service('dom'),
  logger: service('logger'),
  closeOnDestroy: true,
  onerror: function(e) {
    this.logger.execute(e.error);
  },
  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
  },
  willDestroyElement: function() {
    if (this.closeOnDestroy && typeof (this.src || {}).close === 'function') {
      this.src.close();
      this.src.willDestroy();
    }
    this._listeners.remove();
    this._super(...arguments);
  },
  didReceiveAttrs: function() {
    this._listeners.remove();
    if (typeof (this.src || {}).addEventListener === 'function') {
      this._listeners.add(this.src, {
        error: e => {
          try {
            this.onerror(e);
          } catch (err) {
            this.logger.execute(e.error);
          }
        },
      });
    }
  },
});

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

const replace = function(
  obj,
  prop,
  value,
  destroy = (prev = null, value) => (typeof prev === 'function' ? prev() : null)
) {
  const prev = obj[prop];
  if (prev !== value) {
    destroy(prev, value);
  }
  return set(obj, prop, value);
};
export default Component.extend({
  tagName: '',
  dom: service('dom'),
  logger: service('logger'),
  data: service('data-source/service'),
  closeOnDestroy: true,
  onerror: function(e) {
    this.logger.execute(e.error);
  },
  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
  },
  willDestroyElement: function() {
    if (this.closeOnDestroy) {
      this.actions.close.apply(this, []);
    }
    this._listeners.remove();
    this._super(...arguments);
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    // only close and reopen if the uri changes
    // otherwise this will fire whenever the proxies data changes
    if (get(this, 'src.configuration.uri') !== get(this, 'source.configuration.uri')) {
      this.actions.open.apply(this, []);
    }
  },
  actions: {
    open: function() {
      replace(this, 'source', this.data.open(this.src, this), (prev, source) => {
        // Makes sure any previous source (if different) is ALWAYS closed
        if (typeof prev !== 'undefined') {
          this.data.close(prev, this);
        }
      });
      replace(this, 'proxy', this.src, (prev, proxy) => {
        // Makes sure any previous proxy (if different) is ALWAYS closed
        if (typeof prev !== 'undefined') {
          prev.destroy();
        }
      });
      const error = err => {
        try {
          const error = get(err, 'error.errors.firstObject');
          if (get(error || {}, 'status') !== '429') {
            this.onerror(err);
          }
          this.logger.execute(err);
        } catch (err) {
          this.logger.execute(err);
        }
      };
      // set up the listeners (which auto cleanup on component destruction)
      // we only need errors here as this only uses proxies which
      // automatically update their data
      const remove = this._listeners.add(this.source, {
        error: e => {
          error(e);
        },
      });
      replace(this, '_remove', remove);
    },
    close: function() {
      if (typeof this.source !== 'undefined') {
        this.data.close(this.source, this);
        replace(this, '_remove', undefined);
        set(this, 'source', undefined);
      }
      if (typeof this.proxy !== 'undefined') {
        this.proxy.destroy();
      }
    },
  },
});

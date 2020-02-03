import { get, set } from '@ember/object';

const proxies = {};
export default function(ObjProxy, ArrProxy, createListeners) {
  return function(source, data = []) {
    let Proxy = ObjProxy;
    // TODO: Why are these two separate?
    // And when is data ever a string?
    if (typeof data !== 'string' && typeof get(data, 'length') !== 'undefined') {
      data = data.filter(function(item) {
        return !get(item, 'isDestroyed') && !get(item, 'isDeleted') && get(item, 'isLoaded');
      });
    }
    let type = 'object';
    if (typeof data !== 'string' && typeof get(data, 'length') !== 'undefined') {
      Proxy = ArrProxy;
      type = 'array';
    }
    if (typeof proxies[type] === 'undefined') {
      proxies[type] = Proxy.extend({
        closed: false,
        error: null,
        init: function() {
          this.listeners = createListeners();
          this.listeners.add(this._source, 'message', e => set(this, 'content', e.data));
          this.listeners.add(this._source, 'open', () => set(this, 'closed', false));
          this.listeners.add(this._source, 'close', () => set(this, 'closed', true));
          this.listeners.add(this._source, 'error', e => set(this, 'error', e.error));
          this._super(...arguments);
        },
        addEventListener: function(type, handler) {
          // Force use of computed for messages
          // Temporarily disable this restriction
          // if (type !== 'message') {
          this.listeners.add(this._source, type, handler);
          // }
        },
        getCurrentEvent: function() {
          return this._source.getCurrentEvent(...arguments);
        },
        removeEventListener: function() {
          return this._source.removeEventListener(...arguments);
        },
        dispatchEvent: function() {
          return this._source.dispatchEvent(...arguments);
        },
        close: function() {
          return this._source.close(...arguments);
        },
        open: function() {
          return this._source.open(...arguments);
        },
        willDestroy: function() {
          this.listeners.remove();
        },
      });
    }
    return proxies[type].create({
      content: data,
      _source: source,
      configuration: source.configuration,
    });
  };
}

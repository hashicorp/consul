import { get, set } from '@ember/object';

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
    if (typeof data !== 'string' && typeof get(data, 'length') !== 'undefined') {
      Proxy = ArrProxy;
    }
    const proxy = Proxy.create({
      content: data,
      closed: false,
      error: null,
      init: function() {
        this.listeners = createListeners();
        this.listeners.add(source, 'message', e => set(this, 'content', e.data));
        this.listeners.add(source, 'open', () => set(this, 'closed', false));
        this.listeners.add(source, 'close', () => set(this, 'closed', true));
        this.listeners.add(source, 'error', e => set(this, 'error', e.error));
      },
      configuration: source.configuration,
      addEventListener: function(type, handler) {
        // Force use of computed for messages
        // Temporarily disable this restriction
        // if (type !== 'message') {
        this.listeners.add(source, type, handler);
        // }
      },
      getCurrentEvent: function() {
        return source.getCurrentEvent(...arguments);
      },
      removeEventListener: function() {
        return source.removeEventListener(...arguments);
      },
      dispatchEvent: function() {
        return source.dispatchEvent(...arguments);
      },
      close: function() {
        return source.close(...arguments);
      },
      open: function() {
        return source.open(...arguments);
      },
      willDestroy: function() {
        this.listeners.remove();
      },
    });
    return proxy;
  };
}

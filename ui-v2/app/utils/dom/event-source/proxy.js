import { get, set } from '@ember/object';

export default function(ObjProxy, ArrProxy) {
  return function(data, source, listeners) {
    let Proxy = ObjProxy;
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
      init: function() {
        this.listeners = listeners;
        this.listeners.add(source, 'message', e => {
          set(this, 'content', e.data);
        });
      },
      configuration: source.configuration,
      addEventListener: function(type, handler) {
        // Force use of computed for messages
        if (type !== 'message') {
          this.listeners.add(source, type, handler);
        }
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
      reopen: function() {
        return source.reopen(...arguments);
      },
      willDestroy: function() {
        this.listeners.remove();
      },
    });
    return proxy;
  };
}

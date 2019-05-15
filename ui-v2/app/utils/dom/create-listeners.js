class Listeners {
  constructor(listeners = []) {
    this.listeners = listeners;
  }
  add(target, event, handler) {
    let remove;
    if (typeof target === 'function') {
      remove = target;
    } else if (target instanceof Listeners) {
      remove = target.remove.bind(target);
    } else {
      let addEventListener = 'addEventListener';
      let removeEventListener = 'removeEventListener';
      if (typeof target[addEventListener] === 'undefined') {
        addEventListener = 'on';
        removeEventListener = 'off';
      }
      target[addEventListener](event, handler);
      remove = function() {
        target[removeEventListener](event, handler);
        return handler;
      };
    }
    this.listeners.push(remove);
    return () => {
      const pos = this.listeners.findIndex(function(item) {
        return item === remove;
      });
      return this.listeners.splice(pos, 1)[0]();
    };
  }
  remove() {
    const handlers = this.listeners.map(item => item());
    this.listeners.splice(0, this.listeners.length);
    return handlers;
  }
}
export default function(listeners = []) {
  return new Listeners(listeners);
}

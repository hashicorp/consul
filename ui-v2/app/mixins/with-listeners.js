import Mixin from '@ember/object/mixin';

export default Mixin.create({
  init: function() {
    this._super(...arguments);
    this._listeners = [];
  },
  listen: function(target, event, handler) {
    let addEventListener = 'addEventListener';
    let removeEventListener = 'removeEventListener';
    if (typeof target[addEventListener] === 'undefined') {
      addEventListener = 'on';
      removeEventListener = 'off';
    }
    target[addEventListener](event, handler);
    this._listeners.push(function() {
      target[removeEventListener](event, handler);
    });
    return this._listeners.length;
  },
  ignoreAll: function() {
    this._listeners.forEach(function(item) {
      item();
    });
    // temporarily remove all listeners for now
    this._listeners = [];
  },
});

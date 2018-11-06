import Component from '@ember/component';
import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Mixin.create({
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    this._listeners = get(this, 'dom').listeners();
    let method = 'willDestroy';
    if (this instanceof Component) {
      method = 'willDestroyElement';
    }
    const destroy = this[method];
    this[method] = function() {
      destroy(...arguments);
      this.removeListeners();
    };
  },
  listen: function(target, event, handler) {
    return this._listeners.add(...arguments);
  },
  removeListeners: function() {
    return this._listeners.remove(...arguments);
  },
});

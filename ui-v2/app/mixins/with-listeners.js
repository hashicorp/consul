import Controller from '@ember/controller';
import Component from '@ember/component';
import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Mixin.create({
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    this._listeners = get(this, 'dom').listeners();
    let teardown = ['willDestroy'];
    if (this instanceof Component) {
      teardown = ['willDestroyElement'];
    } else if (this instanceof Controller) {
      if (typeof this.reset === 'function') {
        teardown.push('reset');
      }
    }
    teardown.forEach(method => {
      const destroy = this[method];
      this[method] = function() {
        if (typeof destroy === 'function') {
          destroy.apply(this, arguments);
        }
        this.removeListeners();
      };
    });
  },
  listen: function(target, event, handler) {
    return this._listeners.add(...arguments);
  },
  removeListeners: function() {
    return this._listeners.remove(...arguments);
  },
});

import Mixin from '@ember/object/mixin';

import { next } from '@ember/runloop';
import { get } from '@ember/object';
const isOutside = function(element, e) {
  if (element) {
    const isRemoved = !e.target || !document.contains(e.target);
    const isInside = element === e.target || element.contains(e.target);
    return !isRemoved && !isInside;
  } else {
    return false;
  }
};
const handler = function(e) {
  const el = get(this, 'element');
  if (isOutside(el, e)) {
    this.onblur(e);
  }
};
export default Mixin.create({
  init: function() {
    this._super(...arguments);
    this.handler = handler.bind(this);
  },
  onchange: function() {},
  onblur: function() {},
  didInsertElement: function() {
    this._super(...arguments);
    next(this, () => {
      document.addEventListener('click', this.handler);
    });
  },
  willDestroyElement: function() {
    this._super(...arguments);
    document.removeEventListener('click', this.handler);
  },
});

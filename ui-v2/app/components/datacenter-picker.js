import Component from '@ember/component';
import { next } from '@ember/runloop';
import { get } from '@ember/object';

var handler;
const memoized = cb => (handler = cb);
const isOutside = function(element, e) {
  const isRemoved = !e.target || !document.contains(e.target);
  const isInside = element === e.target || element.contains(e.target);
  return !isRemoved && !isInside;
};
const outside = function(el, func) {
  return e => {
    if (isOutside(el, e)) {
      func(e);
    }
  };
};
export default Component.extend({
  tagName: 'ul',
  onchange: function() {},
  onblur: function() {},
  didInsertElement: function() {
    this._super(...arguments);
    next(this, () => {
      document.addEventListener(
        'click',
        handler ? handler : memoized(outside(get(this, 'element'), this.onblur))
      );
    });
  },
  willDestroyElement: function() {
    this._super(...arguments);
    // TODO: Ask if there is a chance that this will be called after `next`
    document.removeEventListener(
      'click',
      handler ? handler : memoized(outside(get(this, 'element'), this.onblur))
    );
    handler = null;
  },
});

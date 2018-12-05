import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { next } from '@ember/runloop';
import { get } from '@ember/object';

// TODO: Potentially move this to dom service
const isOutside = function(element, e, doc = document) {
  if (element) {
    const isRemoved = !e.target || !doc.contains(e.target);
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
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    this.handler = handler.bind(this);
  },
  onchange: function() {},
  onblur: function() {},
  didInsertElement: function() {
    this._super(...arguments);
    const doc = get(this, 'dom').document();
    next(this, () => {
      doc.addEventListener('click', this.handler);
    });
  },
  willDestroyElement: function() {
    this._super(...arguments);
    const doc = get(this, 'dom').document();
    doc.removeEventListener('click', this.handler);
  },
});

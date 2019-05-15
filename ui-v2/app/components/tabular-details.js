import Component from '@ember/component';
import SlotsMixin from 'block-slots';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { subscribe } from 'consul-ui/utils/computed/purify';

let uid = 0;
export default Component.extend(SlotsMixin, {
  dom: service('dom'),
  onchange: function() {},
  init: function() {
    this._super(...arguments);
    set(this, 'uid', uid++);
  },
  inputId: subscribe('name', 'uid', function(name = 'name') {
    return `tabular-details-${name}-toggle-${uid}_`;
  }),
  actions: {
    click: function(e) {
      get(this, 'dom').clickFirstAnchor(e);
    },
    change: function(item, items, e) {
      this.onchange(e, item, items);
    },
  },
});

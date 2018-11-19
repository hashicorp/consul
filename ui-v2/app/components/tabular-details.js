import Component from '@ember/component';
import SlotsMixin from 'block-slots';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Component.extend(SlotsMixin, {
  dom: service('dom'),
  onchange: function() {},
  actions: {
    click: function(e) {
      get(this, 'dom').clickFirstAnchor(e);
    },
    change: function(item, e) {
      this.onchange(e, item);
    },
  },
});

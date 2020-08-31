import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  tagName: '',
  dom: service('dom'),
  didInsertElement: function() {
    this._super(...arguments);
    this.select.addOption(this);
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this.select.removeOption(this);
  },
  actions: {
    click: function(e) {
      this.onclick(e, this.value);
    },
  },
});

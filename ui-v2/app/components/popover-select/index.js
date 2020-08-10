import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  tagName: '',
  dom: service('dom'),
  multiple: false,
  onchange: function() {},
  actions: {
    change: function(option, e) {
      this.onchange(this.dom.setEventTargetProperty(e, 'selected', selected => option));
    },
  },
});

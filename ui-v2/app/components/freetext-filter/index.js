import Component from '@ember/component';
import { inject as service } from '@ember/service';
const ENTER = 13;
export default Component.extend({
  dom: service('dom'),
  tagName: '',
  actions: {
    change: function(e) {
      this.onsearch(
        this.dom.setEventTargetProperty(e, 'value', value => (value === '' ? undefined : value))
      );
    },
    keydown: function(e) {
      if (e.keyCode === ENTER) {
        e.preventDefault();
      }
    },
  },
});

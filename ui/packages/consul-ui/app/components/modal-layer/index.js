import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  dom: service('dom'),
  tagName: '',
  actions: {
    change: function(e) {
      [...this.dom.elements('[name="modal"]')]
        .filter(function(item) {
          return item.getAttribute('id') !== 'modal_close';
        })
        .forEach(function(item, i) {
          if (item.getAttribute('data-checked') === 'true') {
            item.onchange({ target: item });
          }
        });
    },
  },
});

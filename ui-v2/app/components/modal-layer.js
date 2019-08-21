import DomBufferFlushComponent from 'consul-ui/components/dom-buffer-flush';
import { inject as service } from '@ember/service';

export default DomBufferFlushComponent.extend({
  dom: service('dom'),
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

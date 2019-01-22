import Component from 'consul-ui/components/dom-buffer-flush';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Component.extend({
  dom: service('dom'),
  actions: {
    change: function(e) {
      [...get(this, 'dom').elements('[name="modal"]')]
        .filter(function(item) {
          return item.getAttribute('id') !== 'modal_close';
        })
        .forEach(function(item) {
          item.onchange();
        });
    },
  },
});

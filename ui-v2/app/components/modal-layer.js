import Component from 'consul-ui/components/dom-buffer-flush';
import qsaFactory from 'consul-ui/utils/qsa-factory';

const $$ = qsaFactory();

export default Component.extend({
  actions: {
    change: function(e) {
      console.log('change');
      [...$('[name="modal"]')]
        .filter(function(item) {
          return item.getAttribute('id') !== 'modal_close';
        })
        .forEach(function(item) {
          item.onchange();
        });
    },
  },
});

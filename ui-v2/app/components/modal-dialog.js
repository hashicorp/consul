import Component from 'consul-ui/components/dom-buffer';

import templatize from 'consul-ui/utils/templatize';
const $html = document.documentElement;
export default Component.extend({
  // tagName: ''
  didInsertElement: function() {
    this._super(...arguments);
    $html.classList.add(...templatize(['with-modal']));
  },
  didDestroyElement: function() {
    this._super(...arguments);
    $html.classList.remove(...templatize(['with-modal']));
  },
  actions: {
    change: function(e) {
      this.onclose();
    },
    close: function() {
      document.getElementById('modal_close').checked = true;
      this.onclose();
    },
  },
});

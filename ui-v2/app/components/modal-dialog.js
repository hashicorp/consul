import Component from 'consul-ui/components/dom-buffer';
import { get, set } from '@ember/object';
import SlotsMixin from 'ember-block-slots';

import templatize from 'consul-ui/utils/templatize';
const $html = document.documentElement;
export default Component.extend(SlotsMixin, {
  // tagName: ''
  checked: true,
  onclose: function() {},
  didReceiveAttrs: function() {
    this._super(...arguments);
    if (get(this, 'name')) {
      set(this, 'checked', false);
    }
    if (get(this, 'checked')) {
      $html.classList.add(...templatize(['with-modal']));
    }
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

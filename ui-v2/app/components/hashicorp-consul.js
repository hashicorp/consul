import Component from '@ember/component';
import { get, set } from '@ember/object';
const $html = document.documentElement;
const $body = document.body;
export default Component.extend({
  isDropdownVisible: false,
  didInsertElement: function() {
    $html.classList.remove('template-with-vertical-menu');
  },
  actions: {
    dropdown: function(e) {
      if (get(this, 'dcs.length') > 0) {
        set(this, 'isDropdownVisible', !get(this, 'isDropdownVisible'));
      }
    },
    change: function(e) {
      if (e.target.checked) {
        $html.classList.add('template-with-vertical-menu');
        $body.style.height = $html.style.height = window.innerHeight + 'px';
      } else {
        $html.classList.remove('template-with-vertical-menu');
        $body.style.height = $html.style.height = null;
      }
    },
  },
});

import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
export default Component.extend({
  dom: service('dom'),
  // TODO: could this be dom.viewport() ?
  win: window,
  isDropdownVisible: false,
  didInsertElement: function() {
    get(this, 'dom')
      .root()
      .classList.remove('template-with-vertical-menu');
  },
  actions: {
    dropdown: function(e) {
      if (get(this, 'dcs.length') > 0) {
        set(this, 'isDropdownVisible', !get(this, 'isDropdownVisible'));
      }
    },
    change: function(e) {
      const dom = get(this, 'dom');
      const $root = dom.root();
      const $body = dom.element('body');
      if (e.target.checked) {
        $root.classList.add('template-with-vertical-menu');
        $body.style.height = $root.style.height = get(this, 'win').innerHeight + 'px';
      } else {
        $root.classList.remove('template-with-vertical-menu');
        $body.style.height = $root.style.height = null;
      }
    },
  },
});

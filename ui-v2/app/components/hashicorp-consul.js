import Component from '@ember/component';
import { inject as service } from '@ember/service';
export default Component.extend({
  dom: service('dom'),
  didInsertElement: function() {
    this.dom.root().classList.remove('template-with-vertical-menu');
  },
  actions: {
    change: function(e) {
      const win = this.dom.viewport();
      const $root = this.dom.root();
      const $body = this.dom.element('body');
      if (e.target.checked) {
        $root.classList.add('template-with-vertical-menu');
        $body.style.height = $root.style.height = win.innerHeight + 'px';
      } else {
        $root.classList.remove('template-with-vertical-menu');
        $body.style.height = $root.style.height = null;
      }
    },
  },
});

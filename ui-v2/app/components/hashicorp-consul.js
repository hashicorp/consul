import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import env from 'consul-ui/env';

export default Component.extend({
  dom: service('dom'),
  didInsertElement: function() {
    this.dom.root().classList.remove('template-with-vertical-menu');
  },
  canManageNamespaces: computed(function() {
    return env('CONSUL_UI_ENABLE_NAMESPACE_MANAGEMENT');
  }),
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

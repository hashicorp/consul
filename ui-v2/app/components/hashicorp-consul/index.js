import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';

export default Component.extend({
  dom: service('dom'),

  didInsertElement: function() {
    this.dom.root().classList.remove('template-with-vertical-menu');
  },
  // TODO: Right now this is the only place where we need permissions
  // but we are likely to need it elsewhere, so probably need a nice helper
  canManageNspaces: computed('permissions', function() {
    return (
      typeof (this.permissions || []).find(function(item) {
        return item.Resource === 'operator' && item.Access === 'write' && item.Allow;
      }) !== 'undefined'
    );
  }),
  actions: {
    send: function(el, method, ...rest) {
      const component = this.dom.component(el);
      component.actions[method].apply(component, rest || []);
    },
    close: function() {
      this.authForm.reset();
    },
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

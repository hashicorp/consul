import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';

export default Component.extend({
  tagName: '',
  dom: service('dom'),

  didInsertElement: function() {
    this._super(...arguments);
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
    keypressClick: function(e) {
      e.target.dispatchEvent(new MouseEvent('click'));
    },
    open: function() {
      this.authForm.focus();
    },
    close: function() {
      this.authForm.reset();
    },
    reauthorize: function(e) {
      this.modal.close();
      this.onchange(e);
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

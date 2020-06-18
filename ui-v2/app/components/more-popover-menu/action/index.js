import Component from '@ember/component';
import { set } from '@ember/object';

import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  didInsertElement: function() {
    this._super(...arguments);
    this.menu.addSubmenu(this.name);
  },
  didDestroyElement: function() {
    this._super(...arguments);
    this.menu.removeSubmenu(this.name);
  },
  willRender: function() {
    this._super(...arguments);
    set(this, 'hasConfirmation', this._isRegistered('confirmation'));
  },
});

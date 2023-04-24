import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';

import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
  },
  didInsertElement: function() {
    this._super(...arguments);
    this.menu.addSubmenu(this.guid);
  },
  didDestroyElement: function() {
    this._super(...arguments);
    this.menu.removeSubmenu(this.guid);
  },
  willRender: function() {
    this._super(...arguments);
    set(this, 'hasConfirmation', this._isRegistered('confirmation'));
  },
});

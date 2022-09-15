import Component from '@ember/component';
import { set } from '@ember/object';
import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  willRender: function () {
    this._super(...arguments);
    set(this, 'hasHeader', this._isRegistered('header') || this._isRegistered('subheader'));
  },
});

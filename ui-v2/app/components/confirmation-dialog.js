/*eslint ember/closure-actions: "warn"*/
import Component from '@ember/component';

import SlotsMixin from 'block-slots';
import { set } from '@ember/object';

const cancel = function() {
  set(this, 'confirming', false);
};
const execute = function() {
  this.sendAction(...['actionName', ...this['arguments']]);
};
const confirm = function() {
  const [action, ...args] = arguments;
  set(this, 'actionName', action);
  set(this, 'arguments', args);
  set(this, 'confirming', true);
};
export default Component.extend(SlotsMixin, {
  classNameBindings: ['confirming'],
  classNames: ['with-confirmation'],
  message: 'Are you sure?',
  confirming: false,
  permanent: false,
  init: function() {
    this._super(...arguments);
    this.cancel = cancel.bind(this);
    this.execute = execute.bind(this);
    this.confirm = confirm.bind(this);
  },
});

import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import qsaFactory from 'consul-ui/utils/qsa-factory';
const $$ = qsaFactory();

import SlotsMixin from 'ember-block-slots';
const STATE_READY = 'ready';
const STATE_SUCCESS = 'success';
const STATE_ERROR = 'error';
export default Component.extend(SlotsMixin, {
  wait: service('timeout'),
  classNames: ['with-feedback'],
  transition: '',
  transitionClassName: 'feedback-dialog-out',
  state: STATE_READY,
  permanent: true,
  init: function() {
    this._super(...arguments);
    this.success = this._success.bind(this);
    this.error = this._error.bind(this);
  },
  applyTransition: function() {
    const wait = get(this, 'wait').execute;
    const className = get(this, 'transitionClassName');
    wait(0)
      .then(() => {
        set(this, 'transition', className);
        return wait(0);
      })
      .then(() => {
        return new Promise(resolve => {
          $$(`.${className}`, this.element)[0].addEventListener('transitionend', resolve);
        });
      })
      .then(() => {
        set(this, 'transition', '');
        set(this, 'state', STATE_READY);
      });
  },
  _success: function() {
    set(this, 'state', STATE_SUCCESS);
    this.applyTransition();
  },
  _error: function() {
    set(this, 'state', STATE_ERROR);
    this.applyTransition();
  },
});

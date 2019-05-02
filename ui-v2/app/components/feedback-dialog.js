import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import { Promise } from 'rsvp';

import SlotsMixin from 'block-slots';
const STATE_READY = 'ready';
const STATE_SUCCESS = 'success';
const STATE_ERROR = 'error';
export default Component.extend(SlotsMixin, {
  wait: service('timeout'),
  dom: service('dom'),
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
    // TODO: Make 0 default in wait
    wait(0)
      .then(() => {
        set(this, 'transition', className);
        return wait(0);
      })
      .then(() => {
        return new Promise(resolve => {
          get(this, 'dom')
            .element(`.${className}`, this.element)
            .addEventListener('transitionend', resolve);
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

import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';

import SlotsMixin from 'ember-block-slots';
const STATE_READY = 'ready';
const STATE_SUCCESS = 'success';
const STATE_ERROR = 'error';
export default Component.extend(SlotsMixin, {
  wait: service('timeout'),
  interval: null,
  classNames: ['with-feedback'],
  state: STATE_READY,
  permanent: true,
  init: function() {
    this._super(...arguments);
    this.success = this._success.bind(this);
    this.error = this._error.bind(this);
  },
  _success: function() {
    set(this, 'state', STATE_SUCCESS);
    get(this, 'wait')
      .execute(3000, interval => {
        clearInterval(get(this, 'interval'));
        set(this, 'interval', interval);
      })
      .then(() => {
        set(this, 'state', STATE_READY);
      });
  },
  _error: function() {
    set(this, 'state', STATE_ERROR);
    get(this, 'wait')
      .execute(3000, interval => {
        clearInterval(get(this, 'interval'));
        set(this, 'interval', interval);
      })
      .then(() => {
        set(this, 'state', STATE_READY);
      });
  },
});

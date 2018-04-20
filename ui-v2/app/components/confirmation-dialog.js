import Component from '@ember/component';

import SlotsMixin from 'ember-block-slots';
import confirm from 'consul-ui/utils/confirm';
import error from 'consul-ui/utils/error';
import { get, set } from '@ember/object';

export default Component.extend(SlotsMixin, {
  classNames: ['with-confirmation'],
  message: 'Are you sure?',
  init: function() {
    this._super(...arguments);
    this.confirm = function() {
      const [action, ...args] = arguments;
      confirm(get(this, 'message'))
        .then(confirmed => {
          if (confirmed) {
            set(this, 'success', action);
            this.sendAction(...['success', ...args]);
          }
        })
        .catch(error);
    }.bind(this);
  },
  cancel: function() {},
});

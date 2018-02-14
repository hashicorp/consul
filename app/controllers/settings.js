import Controller from '@ember/controller';
import Promise from 'rsvp';
import confirm from 'consul-ui/utils/confirm';
import notify from 'consul-ui/utils/notify';

// emberize this
var dispatch;
var _dispatch = function(prop, val) {
  if (typeof this.props[prop] === 'function') {
    val = this.props[prop].apply(this, [val]);
  }
  return Promise.resolve(this.set(prop, val));
};
export default Controller.extend({
  init: function() {
    dispatch = _dispatch.bind(this);
  },
  props: {
    isLoading: false,
  },
  actions: {
    reset: function() {
      dispatch('isLoading', true)
        .then(function() {
          return confirm('Are you sure you want to reset your settings?');
        })
        .then(function() {
          return dispatch('model.token', '');
        })
        .then(function() {
          return notify('Settings reset', 3000);
        })
        .finally(function() {
          return dispatch('isLoading', false);
        });
    },

    close: function() {
      this.transitionToRoute('index');
    },
  },
});

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import { next } from '@ember/runloop';
export default Route.extend({
  // logger: service('logger'),
  repo: service('dc'),
  actions: {
    error: function(e, transition) {
      let error = {
        status: '',
        detail: 'Error',
      };
      if (e.errors && e.errors[0]) {
        error = e.errors[0];
      }
      // logger(error);
      hash({
        error: error,
        dc: get(this, 'repo').getActive(),
      }).then(model => {
        next(() => {
          this.controllerFor('error').setProperties(model);
        });
      });
      return true;
    },
  },
});

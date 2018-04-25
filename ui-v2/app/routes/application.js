import Route from '@ember/routing/route';
import { next } from '@ember/runloop';
export default Route.extend({
  // logger: service('logger'),
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
      next(() => {
        this.controllerFor('error').set('error', error);
      });
      return true;
    },
  },
});

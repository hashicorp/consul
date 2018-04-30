import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import { next } from '@ember/runloop';
export default Route.extend({
  // logger: service('logger'),
  init: function() {
    document.documentElement.classList.remove('ember-loading');
  },
  repo: service('dc'),
  actions: {
    loading: function(transition, originRoute) {
      let dc = null;
      if (originRoute.routeName !== 'dc') {
        const model = this.modelFor('dc') || { dcs: null, dc: { Name: null } };
        dc = get(this, 'repo').getActive(model.dc.Name, model.dcs);
      }
      hash({
        loading: true,
        dc: dc,
      }).then(model => {
        next(() => {
          const controller = this.controllerFor('application');
          controller.setProperties(model);
          transition.promise.finally(function() {
            controller.setProperties({
              loading: false,
              dc: model.dc,
            });
          });
        });
      });
      return true;
    },
    error: function(e, transition) {
      let error = {
        status: e.code || '',
        message: e.message || 'Error',
      };
      if (e.errors && e.errors[0]) {
        error = e.errors[0];
        error.message = error.title;
      }
      // logger(error);
      hash({
        error: error,
        dc: error.status.toString().indexOf('5') !== 0 ? get(this, 'repo').getActive() : null,
      })
        .then(model => {
          next(() => {
            this.controllerFor('error').setProperties(model);
          });
        })
        .catch(e => {
          next(() => {
            this.controllerFor('error').setProperties({ error: error });
          });
        });
      return true;
    },
  },
});

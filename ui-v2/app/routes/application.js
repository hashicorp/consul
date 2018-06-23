import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import { next } from '@ember/runloop';
const $html = document.documentElement;
const removeLoading = function() {
  return $html.classList.remove('ember-loading');
};
export default Route.extend({
  init: function() {
    this._super(...arguments);
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
        loading: !$html.classList.contains('ember-loading'),
        dc: dc,
      }).then(model => {
        next(() => {
          const controller = this.controllerFor('application');
          controller.setProperties(model);
          transition.promise.finally(function() {
            removeLoading();
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
        message: e.message || e.detail || 'Error',
      };
      if (e.errors && e.errors[0]) {
        error = e.errors[0];
        error.message = error.title || error.detail || 'Error';
      }
      if (error.status === '') {
        error.message = 'Error';
      }
      const model = this.modelFor('dc');
      hash({
        error: error,
        dc:
          error.status.toString().indexOf('5') !== 0
            ? get(this, 'repo').getActive()
            : model && model.dc
              ? model.dc
              : { Name: 'Error' },
        dcs: model && model.dcs ? model.dcs : [],
      })
        .then(model => {
          removeLoading();
          next(() => {
            this.controllerFor('error').setProperties(model);
          });
        })
        .catch(e => {
          removeLoading();
          next(() => {
            this.controllerFor('error').setProperties({ error: error });
          });
        });
      return true;
    },
  },
});

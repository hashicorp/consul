import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { next } from '@ember/runloop';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

const removeLoading = function($from) {
  return $from.classList.remove('ember-loading');
};
export default Route.extend(WithBlockingActions, {
  dom: service('dom'),
  router: service('router'),
  nspacesRepo: service('repository/nspace/disabled'),
  repo: service('repository/dc'),
  settings: service('settings'),
  model: function() {
    return hash({
      router: this.router,
      dcs: this.repo.findAll(),
      nspaces: this.nspacesRepo.findAll().catch(function() {
        return [];
      }),

      // these properties are added to the controller from route/dc
      // as we don't have access to the dc and nspace params in the URL
      // until we get to the route/dc route
      // permissions also requires the dc param

      // dc: null,
      // nspace: null
      // token: null
      // permissions: null
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    loading: function(transition, originRoute) {
      const $root = this.dom.root();
      let dc = null;
      if (originRoute.routeName !== 'dc' && originRoute.routeName !== 'application') {
        const app = this.modelFor('application');
        const model = this.modelFor('dc') || { dc: { Name: null } };
        dc = this.repo.getActive(model.dc.Name, app.dcs);
      }
      hash({
        loading: !$root.classList.contains('ember-loading'),
        dc: dc,
        nspace: this.nspacesRepo.getActive(),
      }).then(model => {
        next(() => {
          const controller = this.controllerFor('application');
          controller.setProperties(model);
          transition.promise.finally(function() {
            removeLoading($root);
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
      // TODO: Normalize all this better
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
      // Try and get the currently attempted dc, whereever that may be
      let model = this.modelFor('dc') || this.modelFor('nspace.dc');
      if (!model) {
        const path = new URL(location.href).pathname
          .substr(this.router.rootURL.length - 1)
          .split('/')
          .slice(1, 3);
        model = {
          nspace: { Name: 'default' },
        };
        if (path[0].startsWith('~')) {
          model.nspace = {
            Name: path.shift(),
          };
        }
        model.dc = {
          Name: path[0],
        };
      }
      const app = this.modelFor('application') || {};
      const dcs = app.dcs || [model.dc];
      const nspaces = app.nspaces || [model.nspace];
      const $root = this.dom.root();
      hash({
        dc:
          error.status.toString().indexOf('5') !== 0
            ? this.repo.getActive(model.dc.Name, dcs)
            : { Name: 'Error' },
        dcs: dcs,
        nspace: model.nspace,
        nspaces: nspaces,
      })
        .then(model => Promise.all([model, this.repo.clearActive()]))
        .then(([model]) => {
          removeLoading($root);
          // we can't use setupController as we received an error
          // so we do it manually instead
          this.controllerFor('application').setProperties(model);
          this.controllerFor('error').setProperties({ error: error });
        })
        .catch(e => {
          removeLoading($root);
          this.controllerFor('error').setProperties({ error: error });
        });
      return true;
    },
  },
});

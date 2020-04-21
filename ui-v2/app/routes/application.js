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
      nspaces: this.nspacesRepo.findAll(),

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
      // TODO: Unfortunately ember will not maintain the correct URL
      // for you i.e. when this happens the URL in your browser location bar
      // will be the URL where you clicked on the link to come here
      // not the URL where you got the 403 response
      // Currently this is dealt with a lot better with the new ACLs system, in that
      // if you get a 403 in the ACLs area, the URL is correct
      // Moving that app wide right now wouldn't be ideal, therefore simply redirect
      // to the ACLs URL instead of maintaining the actual URL, which is better than the old
      // 403 page
      // To note: Consul only gives you back a 403 if a non-existent token has been sent in the header
      // if a token has not been sent at all, it just gives you a 200 with an empty dataset
      // We set a completely null token here, which is different to just deleting a token
      // in that deleting a token means 'logout' whereas setting it to completely null means
      // there was a 403. This is only required to get around the legacy tokens
      // a lot of this can go once we don't support legacy tokens
      if (error.status === '403') {
        return this.settings.persist({
          token: {
            AccessorID: null,
            SecretID: null,
            Namespace: null,
          },
        });
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
          next(() => {
            this.controllerFor('application').setProperties(model);
            this.controllerFor('error').setProperties({ error: error });
          });
        })
        .catch(e => {
          removeLoading($root);
          next(() => {
            this.controllerFor('error').setProperties({ error: error });
          });
        });
      return true;
    },
  },
});

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { next } from '@ember/runloop';
import { Promise } from 'rsvp';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

const removeLoading = function($from) {
  return $from.classList.remove('ember-loading');
};
export default Route.extend(WithBlockingActions, {
  dom: service('dom'),
  nspacesRepo: service('repository/nspace/disabled'),
  repo: service('repository/dc'),
  settings: service('settings'),
  actions: {
    loading: function(transition, originRoute) {
      const $root = this.dom.root();
      let dc = null;
      if (originRoute.routeName !== 'dc') {
        const model = this.modelFor('dc') || { dcs: null, dc: { Name: null } };
        dc = this.repo.getActive(model.dc.Name, model.dcs);
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
      // Try and get the currently attempted dc, whereever that may be
      const model = this.modelFor('dc') || this.modelFor('nspace.dc');
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
      if (error.status === '403') {
        return this.feedback.execute(() => {
          return this.settings.delete('token').then(() => {
            return Promise.reject(this.transitionTo('dc.acls.tokens', model.dc.Name));
          });
        }, 'authorize');
      }
      if (error.status === '') {
        error.message = 'Error';
      }
      const $root = this.dom.root();
      hash({
        error: error,
        nspace: this.nspacesRepo.getActive(),
        dc:
          error.status.toString().indexOf('5') !== 0
            ? this.repo.getActive()
            : model && model.dc
            ? model.dc
            : { Name: 'Error' },
        dcs: model && model.dcs ? model.dcs : [],
      })
        .then(model => Promise.all([model, this.repo.clearActive()]))
        .then(([model]) => {
          removeLoading($root);
          model.nspaces = [model.nspace];
          // we can't use setupController as we received an error
          // so we do it manually instead
          next(() => {
            this.controllerFor('error').setProperties(model);
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

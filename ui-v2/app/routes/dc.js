import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash, Promise } from 'rsvp';
import { get } from '@ember/object';

// TODO: We should potentially move all these nspace related things
// up a level to application.js

const findActiveNspace = function(nspaces, nspace) {
  let found = nspaces.find(function(item) {
    return item.Name === nspace.Name;
  });
  if (typeof found === 'undefined') {
    // if we can't find the nspace that was saved
    // try default
    found = nspaces.find(function(item) {
      return item.Name === 'default';
    });
    // if there is no default just choose the first
    if (typeof found === 'undefined') {
      found = nspaces.firstObject;
    }
  }
  return found;
};
export default Route.extend({
  repo: service('repository/dc'),
  nspacesRepo: service('repository/nspace/disabled'),
  settingsRepo: service('settings'),
  model: function(params) {
    const app = this.modelFor('application');
    return hash({
      nspace: this.nspacesRepo.getActive(),
      token: this.settingsRepo.findBySlug('token'),
      dc: this.repo.findBySlug(params.dc, app.dcs),
    })
      .then(function(model) {
        return hash({
          ...model,
          ...{
            // if there is only 1 namespace then use that
            // otherwise find the namespace object that corresponds
            // to the active one
            nspace:
              app.nspaces.length > 1
                ? findActiveNspace(app.nspaces, model.nspace)
                : app.nspaces.firstObject,
          },
        });
      })
      .then(model => {
        if (get(model, 'token.SecretID')) {
          return hash({
            ...model,
            ...{
              // When disabled nspaces is [], so nspace is undefined
              permissions: this.nspacesRepo.authorize(params.dc, get(model, 'nspace.Name')),
            },
          });
        } else {
          return model;
        }
      });
  },
  setupController: function(controller, model) {
    // the model here is actually required for the entire application
    // but we need to wait until we are in this route so we know what the dc
    // and or nspace is if the below changes please revists the comments
    // in routes/application:model
    this.controllerFor('application').setProperties(model);
  },
  actions: {
    // TODO: This will eventually be deprecated please see
    // https://deprecations.emberjs.com/v3.x/#toc_deprecate-router-events
    willTransition: function(transition) {
      this._super(...arguments);
      if (
        typeof transition !== 'undefined' &&
        (transition.from.name.endsWith('nspaces.create') ||
          transition.from.name.startsWith('nspace.dc.acls.tokens'))
      ) {
        // Only when we create, reload the nspaces in the main menu to update them
        // as we don't block for those
        // And also when we [Use] a token reload the nspaces that you are able to see,
        // including your permissions for being able to manage namespaces
        // Potentially we should just do this on every single transition
        // but then we would need to check to see if nspaces are enabled
        const controller = this.controllerFor('application');
        Promise.all([
          this.nspacesRepo.findAll(),
          this.nspacesRepo.authorize(get(controller, 'dc.Name'), get(controller, 'nspace.Name')),
        ]).then(([nspaces, permissions]) => {
          if (typeof controller !== 'undefined') {
            controller.setProperties({
              nspaces: nspaces,
              permissions: permissions,
            });
          }
        });
      }
    },
  },
});

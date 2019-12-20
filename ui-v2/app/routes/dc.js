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
    const repo = this.repo;
    const nspacesRepo = this.nspacesRepo;
    const settingsRepo = this.settingsRepo;
    return hash({
      dcs: repo.findAll(),
      nspaces: nspacesRepo.findAll(),
      nspace: nspacesRepo.getActive(),
      token: settingsRepo.findBySlug('token'),
    })
      .then(function(model) {
        return hash({
          ...model,
          ...{
            dc: repo.findBySlug(params.dc, model.dcs),
            // if there is only 1 namespace then use that
            // otherwise find the namespace object that corresponds
            // to the active one
            nspace:
              model.nspaces.length > 1
                ? findActiveNspace(model.nspaces, model.nspace)
                : model.nspaces.firstObject,
          },
        });
      })
      .then(function(model) {
        if (get(model, 'token.SecretID')) {
          return hash({
            ...model,
            ...{
              // When disabled nspaces is [], so nspace is undefined
              permissions: nspacesRepo.authorize(params.dc, get(model, 'nspace.Name')),
            },
          });
        } else {
          return model;
        }
      });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
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
        Promise.all([
          this.nspacesRepo.findAll(),
          this.nspacesRepo.authorize(
            get(this.controller, 'dc.Name'),
            get(this.controller, 'nspace.Name')
          ),
        ]).then(([nspaces, permissions]) => {
          if (typeof this.controller !== 'undefined') {
            this.controller.setProperties({
              nspaces: nspaces,
              permissions: permissions,
            });
          }
        });
      }
    },
  },
});

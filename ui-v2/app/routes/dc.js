import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
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
              permissions: nspacesRepo.authorize(params.dc, model.nspace.Name),
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
      // This will refresh both dcs and nspaces on any route transition
      // under here
      if (typeof transition !== 'undefined' && transition.from.name.endsWith('nspaces.create')) {
        this.refresh();
      }
    },
  },
});

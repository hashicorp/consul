import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

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
  nspaceRepo: service('repository/nspace/disabled'),
  model: function(params) {
    const repo = this.repo;
    const nspaceRepo = this.nspaceRepo;
    return hash({
      dcs: repo.findAll(),
      nspaces: nspaceRepo.findAll(),
      nspace: this.nspaceRepo.getActive(),
    }).then(function(model) {
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

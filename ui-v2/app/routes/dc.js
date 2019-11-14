import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

// TODO: We should potentially move all this up a level to application.js
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
              ? model.nspaces.find(function(item) {
                  return item.Name === model.nspace.Name;
                })
              : model.nspaces.firstObject,
        },
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});

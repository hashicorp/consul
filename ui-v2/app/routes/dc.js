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
    const nspace = this.nspaceRepo.getActive();
    return hash({
      dcs: repo.findAll(),
      nspaces: nspaceRepo.findAll(),
    }).then(function(model) {
      return hash({
        ...model,
        ...{
          dc: repo.findBySlug(params.dc, model.dcs),
          nspace:
            model.nspaces.length > 0
              ? model.nspaces.find(function(item) {
                  return item.Name === nspace.Name;
                })
              : null,
        },
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});

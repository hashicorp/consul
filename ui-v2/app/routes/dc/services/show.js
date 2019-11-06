import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('repository/service'),
  chainRepo: service('repository/discovery-chain'),
  settings: service('settings'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    const repo = this.repo;
    const chainRepo = this.chainRepo;
    const settings = this.settings;
    const dc = this.modelFor('dc').dc.Name;
    return hash({
      item: repo.findBySlug(params.name, dc),
      chain: chainRepo.findBySlug(params.name, dc),
      urls: settings.findBySlug('urls'),
      dc: dc,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});

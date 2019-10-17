import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('repository/node'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    return hash({
      items: this.repo.findAllByDatacenter(dc, this.modelFor('nspace').nspace.substr(1)),
      leader: this.repo.findByLeader(dc),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});

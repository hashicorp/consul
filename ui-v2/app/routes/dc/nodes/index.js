import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('repository/node'),
  data: service('data-source/service'),
  queryParams: {
    search: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    return hash({
      items: this.data.source(uri => uri`/${nspace}/${dc}/nodes`),
      leader: this.repo.findByLeader(dc),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});

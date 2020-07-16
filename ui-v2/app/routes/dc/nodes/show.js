import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  data: service('data-source/service'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    const name = params.name;
    return hash({
      dc: dc,
      nspace: nspace,
      item: this.data.source(uri => uri`/${nspace}/${dc}/node/${name}`),
    }).then(model => {
      return hash({
        ...model,
        tomography: this.data.source(uri => uri`/${nspace}/${dc}/coordinates/for-node/${name}`),
        sessions: this.data.source(uri => uri`/${nspace}/${dc}/sessions/for-node/${name}`),
      });
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});

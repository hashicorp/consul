import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

export default class ShowRoute extends Route {
  @service('data-source/service') data;

  model(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.optionalParams().nspace;
    const name = params.name;
    return hash({
      dc: dc,
      nspace: nspace,
      item: this.data.source(uri => uri`/${nspace}/${dc}/node/${name}`),
    }).then(model => {
      return hash({
        ...model,
        tomography: this.data.source(uri => uri`/${nspace}/${dc}/coordinates/for-node/${name}`),
      });
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

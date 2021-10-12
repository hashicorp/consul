import Route from 'consul-ui/routing/route';
import { get } from '@ember/object';

export default class EditRoute extends Route {
  model(params, transition) {
    return {
      nspace: '*',
      dc: this.paramsFor('dc').dc,
      service: this.paramsFor('dc.services.show').name,
      src: params.intention_id,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

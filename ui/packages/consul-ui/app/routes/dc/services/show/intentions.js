import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class IntentionsRoute extends Route {
  @service('routlet') routlet;

  async model(params, transition) {
    await this.routlet.ready();
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class IntentionsRoute extends Route {
  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

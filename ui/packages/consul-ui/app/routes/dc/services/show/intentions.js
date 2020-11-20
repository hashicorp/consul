import Route from 'consul-ui/routing/route';

export default class IntentionsRoute extends Route {
  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

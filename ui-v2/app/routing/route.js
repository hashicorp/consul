import Route from '@ember/routing/route';

/**
 * Set the routeName for the controller so that it is
 * available in the template for the route/controller
 */
export default class BaseRoute extends Route {
  setupController(controller, model) {
    controller.routeName = this.routeName;
    super.setupController(...arguments);
  }
}

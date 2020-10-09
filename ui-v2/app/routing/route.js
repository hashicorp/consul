import Route from '@ember/routing/route';
import { setProperties } from '@ember/object';

/**
 * Set the routeName for the controller so that it is
 * available in the template for the route/controller
 */
export default class BaseRoute extends Route {
  setupController(controller, model) {
    setProperties(controller, {
      routeName: this.routeName,
    });
    super.setupController(...arguments);
  }
}

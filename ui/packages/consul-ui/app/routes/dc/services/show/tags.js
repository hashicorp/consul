import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class TagsRoute extends Route {
  async model() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

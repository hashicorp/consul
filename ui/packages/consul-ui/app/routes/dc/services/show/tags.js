import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class TagsRoute extends Route {
  @service('routlet') routlet;
  async model() {
    await this.routlet.ready();

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

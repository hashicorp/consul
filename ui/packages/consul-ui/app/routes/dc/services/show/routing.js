import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default class RoutingRoute extends Route {
  @service('data-source/service') data;

  async model(params, transition) {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    const model = this.modelFor(parent);
    return {
      ...model,
      chain: await this.data.source(
        uri => uri`/${model.nspace}/${model.dc.Name}/discovery-chain/${model.slug}`
      ),
    };
  }

  async afterModel(model, transition) {
    if (!get(model, 'chain')) {
      const parent = this.routeName
        .split('.')
        .slice(0, -1)
        .join('.');
      this.replaceWith(parent);
    }
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

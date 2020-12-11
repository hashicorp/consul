import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class TopologyRoute extends Route {
  @service('ui-config') config;
  @service('env') env;

  model() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');

    return {
      ...this.modelFor(parent),
      hasMetricsProvider: !!this.config.get().metrics_provider,
      isRemoteDC: this.env.var('CONSUL_DATACENTER_LOCAL') !== this.modelFor('dc').dc.Name,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

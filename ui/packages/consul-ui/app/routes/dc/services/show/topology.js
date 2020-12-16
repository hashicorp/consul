import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { get, action } from '@ember/object';

export default class TopologyRoute extends Route {
  @service('ui-config') config;
  @service('env') env;
  @service('data-source/service') data;
  @service('repository/intention') repo;

  @action
  async createIntention(source, destination) {
    const model = this.repo.create({
      Datacenter: source.Datacenter,
      SourceName: source.Name,
      SourceNS: source.Namespace || 'default',
      DestinationName: destination.Name,
      DestinationNS: destination.Namespace || 'default',
      Action: 'allow',
    });
    await this.repo.persist(model);
    this.refresh();
  }

  async model() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    const model = this.modelFor(parent);
    const dc = get(model, 'dc');
    const nspace = get(model, 'nspace');

    const item = get(model, 'items.firstObject');
    if (get(item, 'IsMeshOrigin')) {
      let kind = get(item, 'Service.Kind');
      if (typeof kind === 'undefined') {
        kind = '';
      }
      model.topology = await this.data.source(
        uri => uri`/${nspace}/${dc.Name}/topology/${model.slug}/${kind}`
      );
    }
    return {
      ...model,
      hasMetricsProvider: !!this.config.get().metrics_provider,
      isRemoteDC: this.env.var('CONSUL_DATACENTER_LOCAL') !== this.modelFor('dc').dc.Name,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

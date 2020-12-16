import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { get, action } from '@ember/object';

export default class TopologyRoute extends Route {
  @service('ui-config') config;
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
    let kind = get(item, 'Service.Kind');
    if (typeof kind === 'undefined') {
      kind = '';
    }
    const topology = await this.data.source(
      uri => uri`/${nspace}/${dc.Name}/topology/${model.slug}/${kind}`
    );
    let hasMetricsProvider = await this.config.findByPath('metrics_provider');
    hasMetricsProvider = !!hasMetricsProvider;

    return {
      ...model,
      topology,
      hasMetricsProvider,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

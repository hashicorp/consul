import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { set, get, action } from '@ember/object';

export default class TopologyRoute extends Route {
  @service('ui-config') config;
  @service('data-source/service') data;
  @service('repository/intention') repo;
  @service('feedback') feedback;

  @action
  async createIntention(source, destination) {
    // begin with a create action as it makes more sense if the we can't even
    // get a list of intentions
    let notification = this.feedback.notification('create');
    try {
      // intentions will be a proxy object
      let intentions = await this.intentions;
      let intention = intentions.find(item => {
        return (
          item.Datacenter === source.Datacenter &&
          item.SourceName === source.Name &&
          item.SourceNS === source.Namespace &&
          item.DestinationName === destination.Name &&
          item.DestinationNS === destination.Namespace
        );
      });
      if (typeof intention === 'undefined') {
        intention = this.repo.create({
          Datacenter: source.Datacenter,
          SourceName: source.Name,
          SourceNS: source.Namespace || 'default',
          DestinationName: destination.Name,
          DestinationNS: destination.Namespace || 'default',
        });
      } else {
        // we found an intention in the find higher up, so we are updating
        notification = this.feedback.notification('update');
      }
      set(intention, 'Action', 'allow');
      await this.repo.persist(intention);
      notification.success(intention);
    } catch (e) {
      notification.error(e);
    }
    this.refresh();
  }

  afterModel(model, transition) {
    this.intentions = this.data.source(
      uri => uri`/${model.nspace}/${model.dc.Name}/intentions/for-service/${model.slug}`
    );
  }

  async deactivate(transition) {
    const intentions = await this.intentions;
    intentions.destroy();
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

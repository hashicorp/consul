import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { get } from '@ember/object';
import { action, setProperties } from '@ember/object';

export default class ShowRoute extends Route {
  @service('data-source/service') data;
  @service('repository/intention') repo;
  @service('ui-config') config;

  @action
  async createIntention(source, destination) {
    const intention = service.Intention;
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

  async model(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    const slug = params.name;

    let chain = null;
    let topology = null;
    let proxies = [];

    const urls = this.config.get().dashboard_url_templates;
    const items = await this.data.source(
      uri => uri`/${nspace}/${dc}/service-instances/for-service/${params.name}`
    );

    const item = get(items, 'firstObject');
    if (get(item, 'IsOrigin')) {
      chain = await this.data.source(uri => uri`/${nspace}/${dc}/discovery-chain/${params.name}`);
      proxies = await this.data.source(
        uri => uri`/${nspace}/${dc}/proxies/for-service/${params.name}`
      );

      if (get(item, 'IsMeshOrigin')) {
        let kind = get(item, 'Service.Kind');
        if (typeof kind === 'undefined') {
          kind = '';
        }
        topology = await this.data.source(
          uri => uri`/${nspace}/${dc}/topology/${params.name}/${kind}`
        );
      }
    }

    return {
      dc,
      nspace,
      slug,
      items,
      urls,
      chain,
      proxies,
      topology,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

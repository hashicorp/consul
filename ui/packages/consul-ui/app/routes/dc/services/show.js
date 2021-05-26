import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { get } from '@ember/object';

export default class ShowRoute extends Route {
  @service('data-source/service') data;
  @service('ui-config') config;

  async model(params, transition) {
    const dc = this.modelFor('dc').dc;
    const nspace = this.optionalParams().nspace;
    const slug = params.name;

    let chain;
    let proxies = [];

    const urls = await this.config.findByPath('dashboard_url_templates');
    const items = await this.data.source(
      uri => uri`/${nspace}/${dc.Name}/service-instances/for-service/${params.name}`
    );

    const item = get(items, 'firstObject');
    if (get(item, 'IsOrigin')) {
      proxies = this.data.source(
        uri => uri`/${nspace}/${dc.Name}/proxies/for-service/${params.name}`
      );
      // TODO: Temporary ping to see if a dc is MeshEnabled which we use in
      // order to decide whether to show certain tabs in the template. This is
      // a bit of a weird place to do this but we are trying to avoid wasting
      // HTTP requests and as disco chain is the most likely to be reused, we
      // use that endpoint here. Eventually if we have an endpoint specific to
      // a dc that gives us more DC specific info we can use that instead
      // higher up the routing hierarchy instead.
      chain = this.data.source(uri => uri`/${nspace}/${dc.Name}/discovery-chain/${params.name}`);
      [chain, proxies] = await Promise.all([chain, proxies]);
      // we close the chain for now, if you enter the routing tab before the
      // EventSource comes around to request again, this one will just be
      // reopened and reused
      chain.close();
    }
    return {
      dc,
      nspace,
      slug,
      items,
      urls,
      chain,
      proxies,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

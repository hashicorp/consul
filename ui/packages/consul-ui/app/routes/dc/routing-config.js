import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class RoutingConfigRoute extends Route {
  @service('data-source/service') data;

  async model(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.optionalParams().nspace;
    const name = params.name;

    return {
      dc: dc,
      nspace: nspace,
      slug: name,
      chain: await this.data.source(uri => uri`/${nspace}/${dc}/discovery-chain/${params.name}`),
    };
  }
}

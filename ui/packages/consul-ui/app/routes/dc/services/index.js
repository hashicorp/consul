import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

export default class IndexRoute extends Route {
  @service('data-source/service') data;
  @service('routlet') routlet;

  queryParams = {
    sortBy: 'sort',
    status: 'status',
    source: 'source',
    kind: 'kind',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Tags']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  async model(params, transition) {
    const nspace = this.modelFor('nspace').nspace.substr(1);
    const dc = this.modelFor('dc').dc.Name;
    const items = this.data.source(uri => uri`/${nspace}/${dc}/services`);
    await this.routlet.ready();
    return {
      dc,
      nspace,
      items: await items,
      searchProperties: this.queryParams.searchproperty.empty[0],
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

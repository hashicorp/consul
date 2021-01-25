import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

export default class IndexRoute extends Route {
  @service('data-source/service') data;

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
    const items = await this.data.source(uri => uri`/${nspace}/${dc}/services`);
    return {
      dc,
      nspace,
      items,
      searchProperties: this.queryParams.searchproperty.empty[0],
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

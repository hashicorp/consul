import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

export default class IndexRoute extends Route {
  @service('data-source/service') data;
  @service('routlet') routlet;

  queryParams = {
    sortBy: 'sort',
    status: 'status',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Node', 'Address', 'Meta']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  async model(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    const items = this.data.source(uri => uri`/${nspace}/${dc}/nodes`);
    const leader = this.data.source(uri => uri`/${nspace}/${dc}/leader`);
    await this.routlet.ready();
    return {
      items: await items,
      leader: await leader,
      searchProperties: this.queryParams.searchproperty.empty[0],
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

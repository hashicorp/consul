import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

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

  model(params) {
    const nspace = this.modelFor('nspace').nspace.substr(1);
    const dc = this.modelFor('dc').dc.Name;
    return hash({
      nspace: nspace,
      dc: dc,
      items: this.data.source(uri => uri`/${nspace}/${dc}/services`),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

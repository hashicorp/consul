import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

export default class IndexRoute extends Route {
  @service('data-source/service') data;

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

  model(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    return hash({
      items: this.data.source(uri => uri`/${nspace}/${dc}/nodes`),
      leader: this.data.source(uri => uri`/${nspace}/${dc}/leader`),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

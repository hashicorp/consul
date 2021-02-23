import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class InstancesRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    status: 'status',
    source: 'source',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Tags', 'ID', 'Address', 'Port', 'Service.Meta', 'Node.Meta']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  async model() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return {
      ...this.modelFor(parent),
      searchProperties: this.queryParams.searchproperty.empty[0],
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}

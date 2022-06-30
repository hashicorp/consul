import Route from 'consul-ui/routing/route';
import { action } from '@ember/object';

export default class PeersRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}

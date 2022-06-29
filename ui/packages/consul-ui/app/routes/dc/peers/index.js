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

  async model() {
    const parent = await super.model();
    return this.store.findAll('peer').then(peers => {
      return {
        ...parent,
        peers,
        loadPeers: this.loadPeers,
      };
    });
  }

  @action loadPeers() {
    return this.store.findAll('peer');
  }
}

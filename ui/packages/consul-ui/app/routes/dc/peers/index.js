import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class PeersRoute extends Route {
  model() {
    return this.store.findAll('peer').then(peers => {
      return {
        peers,
        loadPeers: this.loadPeers,
      };
    });
  }

  @action loadPeers() {
    return this.store.findAll('peer');
  }
}

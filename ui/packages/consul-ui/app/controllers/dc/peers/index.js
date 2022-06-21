import Controller from '@ember/controller';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class PeersController extends Controller {
  queryParams = ['filter'];

  @tracked filter = '';

  get peers() {
    return this.model.peers;
  }

  get filteredPeers() {
    const { peers, filter } = this;

    if (filter) {
      const filterRegex = new RegExp(`${filter}`, 'gi');

      return peers.filter(peer => peer.Name.match(filterRegex));
    }

    return peers;
  }

  @action handleSearchChanged(newSearchTerm) {
    this.filter = newSearchTerm;
  }
}

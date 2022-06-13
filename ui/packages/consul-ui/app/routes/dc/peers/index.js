import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export default class PeersRoute extends Route {
  model() {
    return this.store.findAll('peer');
  }
}

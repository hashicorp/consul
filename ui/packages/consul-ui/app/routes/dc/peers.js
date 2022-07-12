import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default class PeersRoute extends Route {
  @service abilities;

  beforeModel() {
    if (!this.abilities.can('use peers')) {
      this.transitionTo('dc.services.index');
    }
  }
}

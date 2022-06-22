import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';

export default class PeersRoute extends Route {
  @service features;

  beforeModel() {
    if (!this.features.isEnabled('peering')) {
      this.transitionTo('dc.services.index');
    }
  }
}

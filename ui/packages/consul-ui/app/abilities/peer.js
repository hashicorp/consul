import BaseAbility from 'consul-ui/abilities/base';
import { inject as service } from '@ember/service';

export default class PeerAbility extends BaseAbility {
  @service('env') env;

  resource = 'peering';
  segmented = false;

  get isLinkable() {
    return this.canDelete;
  }
  get canDelete() {
    // TODO: Need to confirm these states
    return !['DELETING', 'TERMINATED', 'UNDEFINED'].includes(this.item.State) && super.canDelete;
  }

  get canUse() {
    return this.env.var('CONSUL_PEERING_ENABLED');
  }
}

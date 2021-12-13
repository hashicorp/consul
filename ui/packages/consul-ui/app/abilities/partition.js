import BaseAbility from 'consul-ui/abilities/base';
import { inject as service } from '@ember/service';

export default class PartitionAbility extends BaseAbility {
  @service('env') env;

  resource = 'operator';
  segmented = false;

  get isLinkable() {
    return !this.item.DeletedAt;
  }

  get canManage() {
    return this.canCreate;
  }

  get canDelete() {
    return this.item.Name !== 'default' && super.canDelete;
  }

  get canChoose() {
    if (typeof this.dc === 'undefined') {
      return false;
    }
    return this.canUse && this.dc.Primary;
  }

  get canSee() {
    if (typeof this.items !== 'undefined' && this.items.length > 0) {
      return true;
    }
    return false;
  }

  get canUse() {
    return this.env.var('CONSUL_PARTITIONS_ENABLED');
  }
}

import BaseAbility from './base';
import { inject as service } from '@ember/service';

export default class NspaceAbility extends BaseAbility {
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
    return this.canUse;
  }

  get canSee() {
    if (typeof this.items !== 'undefined' && this.items.length > 0) {
      return true;
    }
    return false;
  }

  get canUse() {
    return this.env.var('CONSUL_NSPACES_ENABLED');
  }
}

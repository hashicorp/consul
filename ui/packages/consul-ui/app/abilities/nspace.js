import BaseAbility from './base';
import { inject as service } from '@ember/service';

export default class NspaceAbility extends BaseAbility {
  @service('env') env;

  resource = 'operator';
  segmented = false;

  get canManage() {
    return this.canCreate;
  }

  get canDelete() {
    return this.item.Name !== 'default' && super.canDelete;
  }

  get canChoose() {
    return this.canUse;
  }

  get canUse() {
    return this.env.var('CONSUL_NSPACES_ENABLED');
  }
}

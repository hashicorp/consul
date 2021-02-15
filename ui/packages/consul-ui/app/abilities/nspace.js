import BaseAbility from './base';
import { inject as service } from '@ember/service';

export default class NspaceAbility extends BaseAbility {
  @service('env') env;

  resource = 'operator';
  segmented = false;

  get canManage() {
    return this.canCreate;
  }

  get canChoose() {
    return this.env.var('CONSUL_NSPACES_ENABLED') && this.nspaces.length > 0;
  }
}

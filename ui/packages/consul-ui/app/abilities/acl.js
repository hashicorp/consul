import BaseAbility from './base';
import { inject as service } from '@ember/service';

export default class ACLAbility extends BaseAbility {
  @service('env') env;

  resource = 'acl';
  segmented = false;

  get canRead() {
    return this.env.var('CONSUL_ACLS_ENABLED') && super.canRead;
  }

  get canDuplicate() {
    return this.env.var('CONSUL_ACLS_ENABLED') && super.canWrite;
  }

  get canDelete() {
    return this.env.var('CONSUL_ACLS_ENABLED') && this.item.ID !== 'anonymous' && super.canWrite;
  }

  get canUse() {
    return this.env.var('CONSUL_ACLS_ENABLED');
  }
}

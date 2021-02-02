import BaseAbility from './base';
import { inject as service } from '@ember/service';

// ACL ability covers all of the ACL things, like tokens, policies, roles and
// auth methods and this therefore should not be deleted once we remove the on
// legacy ACLs related classes
export default class ACLAbility extends BaseAbility {
  @service('env') env;

  resource = 'acl';

  get canRead() {
    return this.env.var('CONSUL_ACLS_ENABLED') && super.canRead;
  }
}

import BaseAbility from './base';
import { inject as service } from '@ember/service';
export default class AuthenticateAbility extends BaseAbility {
  @service('env') env;
  get can() {
    return this.env.var('CONSUL_ACLS_ENABLED');
  }
}

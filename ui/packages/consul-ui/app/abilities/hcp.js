import BaseAbility from './base';
import { inject as service } from '@ember/service';

export default class HcpAbility extends BaseAbility {
  @service('env') env;

  get is() {
    return false;
    // return this.env.var('CONSUL_HCP');
  }
}

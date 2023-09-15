import BaseAbility from './base';
import { inject as service } from '@ember/service';

export default class LicenseAbility extends BaseAbility {
  resource = 'operator';
  segmented = false;

  @service('env') env;

  get canRead() {
    return this.env.var('CONSUL_NSPACES_ENABLED') && super.canRead;
  }
}

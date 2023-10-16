import BaseAbility from './base';
import { inject as service } from '@ember/service';

export default class OverviewAbility extends BaseAbility {
  @service('env') env;

  resource = 'operator';
  segmented = false;
  get canAccess() {
    return !this.env.var('CONSUL_HCP_ENABLED') && this.canRead;
  }
}

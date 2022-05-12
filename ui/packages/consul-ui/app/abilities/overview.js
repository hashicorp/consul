import BaseAbility from './base';

export default class OverviewAbility extends BaseAbility {
  resource = 'operator';
  segmented = false;
  get canAccess() {
    return this.canRead;
  }
}

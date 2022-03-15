import BaseAbility from './base';

export default class OverviewAbility extends BaseAbility {
  get canAccess() {
    return ['read services', 'read nodes', 'read license']
      .some(item => this.permissions.can(item))
  }
}

import BaseAbility from './base';

export default class ServiceAbility extends BaseAbility {
  resource = 'service';

  get isLinkable() {
    return this.item.InstanceCount > 0;
  }
}

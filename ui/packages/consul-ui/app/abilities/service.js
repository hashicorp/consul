import BaseAbility from './base';

export default class ServiceAbility extends BaseAbility {
  resource = 'service';

  get canReadIntention() {
    const found = this.item.Resources.find(item => item.Resource === 'intention' && item.Access === 'read' && item.Allow === true);
    return typeof found !== 'undefined';
  }

  get canWriteIntention() {
    const found = this.item.Resources.find(item => item.Resource === 'intention' && item.Access === 'write' && item.Allow === true);
    return typeof found !== 'undefined';
  }

  get canCreateIntention() {
    return this.canWriteIntention;
  }

  get canUpdateIntention() {
    return this.canWriteIntention;
  }
}
